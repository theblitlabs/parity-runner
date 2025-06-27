package training

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// DataLoader handles loading training data from IPFS/Filecoin
type DataLoader struct {
	ipfsGateway string
}

// PartitionConfig defines how to partition data for federated learning
type PartitionConfig struct {
	Strategy     string  `json:"strategy"`      // "random", "stratified", "sequential", "iid", "non_iid"
	TotalParts   int     `json:"total_parts"`   // Total number of participants
	PartIndex    int     `json:"part_index"`    // This participant's index (0-based)
	Alpha        float64 `json:"alpha"`         // Dirichlet distribution parameter for non-IID
	MinSamples   int     `json:"min_samples"`   // Minimum samples per participant
	OverlapRatio float64 `json:"overlap_ratio"` // Overlap between partitions (0.0 = no overlap, 0.1 = 10% overlap)
}

// NewDataLoader creates a new DataLoader instance
func NewDataLoader(ipfsGateway string) *DataLoader {
	if ipfsGateway == "" {
		ipfsGateway = "https://ipfs.io/ipfs/" // Default gateway
	}
	return &DataLoader{
		ipfsGateway: ipfsGateway,
	}
}

// LoadData loads data from IPFS/Filecoin based on CID and format
func (d *DataLoader) LoadData(ctx context.Context, cid string, format string) ([][]float64, []float64, error) {
	return d.LoadPartitionedData(ctx, cid, format, nil)
}

// LoadPartitionedData loads and partitions data for federated learning
func (d *DataLoader) LoadPartitionedData(ctx context.Context, cid string, format string, partitionConfig *PartitionConfig) ([][]float64, []float64, error) {
	url := d.ipfsGateway + cid

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	defer resp.Body.Close()

	var features [][]float64
	var labels []float64

	switch strings.ToLower(format) {
	case "csv":
		features, labels, err = d.parseCSV(resp.Body)
	case "json":
		features, labels, err = d.parseJSON(resp.Body)
	default:
		return nil, nil, fmt.Errorf("unsupported data format: %s", format)
	}

	if err != nil {
		return nil, nil, err
	}

	// Apply data partitioning if config is provided
	if partitionConfig != nil {
		return d.partitionData(features, labels, partitionConfig)
	}

	return features, labels, nil
}

func (d *DataLoader) partitionData(features [][]float64, labels []float64, config *PartitionConfig) ([][]float64, []float64, error) {
	if config.TotalParts <= 0 || config.PartIndex < 0 || config.PartIndex >= config.TotalParts {
		return nil, nil, fmt.Errorf("invalid partition configuration")
	}

	totalSamples := len(features)
	if totalSamples == 0 {
		return features, labels, nil
	}

	switch strings.ToLower(config.Strategy) {
	case "random", "iid":
		return d.randomPartition(features, labels, config)
	case "sequential":
		return d.sequentialPartition(features, labels, config)
	case "stratified":
		return d.stratifiedPartition(features, labels, config)
	case "non_iid", "dirichlet":
		return d.nonIIDPartition(features, labels, config)
	case "label_skew":
		return d.labelSkewPartition(features, labels, config)
	default:
		return nil, nil, fmt.Errorf("unsupported partitioning strategy: %s", config.Strategy)
	}
}

// randomPartition creates random IID partitions
func (d *DataLoader) randomPartition(features [][]float64, labels []float64, config *PartitionConfig) ([][]float64, []float64, error) {
	totalSamples := len(features)

	// Create indices and shuffle them
	indices := make([]int, totalSamples)
	for i := range indices {
		indices[i] = i
	}
	rand.Shuffle(len(indices), func(i, j int) { indices[i], indices[j] = indices[j], indices[i] })

	// Calculate partition size with overlap
	baseSize := totalSamples / config.TotalParts
	overlapSize := int(float64(baseSize) * config.OverlapRatio)
	partitionSize := baseSize + overlapSize

	// Ensure minimum samples
	if config.MinSamples > 0 && partitionSize < config.MinSamples {
		partitionSize = config.MinSamples
	}

	// Calculate start and end indices for this partition
	start := config.PartIndex * baseSize
	end := start + partitionSize

	// Handle boundary conditions
	if end > totalSamples {
		end = totalSamples
	}
	if start >= totalSamples {
		start = totalSamples - 1
	}

	// Extract partition indices
	partitionIndices := indices[start:end]

	// Create partitioned data
	partFeatures := make([][]float64, len(partitionIndices))
	partLabels := make([]float64, len(partitionIndices))

	for i, idx := range partitionIndices {
		partFeatures[i] = features[idx]
		partLabels[i] = labels[idx]
	}

	return partFeatures, partLabels, nil
}

// sequentialPartition creates sequential partitions
func (d *DataLoader) sequentialPartition(features [][]float64, labels []float64, config *PartitionConfig) ([][]float64, []float64, error) {
	totalSamples := len(features)
	partitionSize := totalSamples / config.TotalParts

	// Ensure minimum samples
	if config.MinSamples > 0 && partitionSize < config.MinSamples {
		partitionSize = config.MinSamples
	}

	start := config.PartIndex * partitionSize
	end := start + partitionSize

	// Handle last partition getting remaining samples
	if config.PartIndex == config.TotalParts-1 {
		end = totalSamples
	}

	if start >= totalSamples {
		return [][]float64{}, []float64{}, nil
	}
	if end > totalSamples {
		end = totalSamples
	}

	return features[start:end], labels[start:end], nil
}

// stratifiedPartition maintains class distribution across partitions
func (d *DataLoader) stratifiedPartition(features [][]float64, labels []float64, config *PartitionConfig) ([][]float64, []float64, error) {
	// Group samples by class
	classGroups := make(map[float64][]int)
	for i, label := range labels {
		classGroups[label] = append(classGroups[label], i)
	}

	var partitionIndices []int

	// For each class, take approximately equal portion
	for _, indices := range classGroups {
		rand.Shuffle(len(indices), func(i, j int) { indices[i], indices[j] = indices[j], indices[i] })

		classSize := len(indices)
		partSize := classSize / config.TotalParts

		start := config.PartIndex * partSize
		end := start + partSize

		// Handle last partition
		if config.PartIndex == config.TotalParts-1 {
			end = classSize
		}

		if start < classSize {
			if end > classSize {
				end = classSize
			}
			partitionIndices = append(partitionIndices, indices[start:end]...)
		}
	}

	// Shuffle the final partition to avoid class clustering
	rand.Shuffle(len(partitionIndices), func(i, j int) {
		partitionIndices[i], partitionIndices[j] = partitionIndices[j], partitionIndices[i]
	})

	// Create partitioned data
	partFeatures := make([][]float64, len(partitionIndices))
	partLabels := make([]float64, len(partitionIndices))

	for i, idx := range partitionIndices {
		partFeatures[i] = features[idx]
		partLabels[i] = labels[idx]
	}

	return partFeatures, partLabels, nil
}

// nonIIDPartition creates non-IID partitions using Dirichlet distribution
func (d *DataLoader) nonIIDPartition(features [][]float64, labels []float64, config *PartitionConfig) ([][]float64, []float64, error) {
	// Group samples by class
	classGroups := make(map[float64][]int)
	for i, label := range labels {
		classGroups[label] = append(classGroups[label], i)
	}

	alpha := config.Alpha
	if alpha <= 0 {
		return nil, nil, fmt.Errorf("alpha parameter must be positive for non-IID partitioning, got %f", alpha)
	}

	var partitionIndices []int

	// For each class, use Dirichlet distribution to assign samples
	for _, indices := range classGroups {
		rand.Shuffle(len(indices), func(i, j int) { indices[i], indices[j] = indices[j], indices[i] })

		classSize := len(indices)

		// Simulate Dirichlet distribution with given alpha
		// Higher alpha = more uniform, lower alpha = more skewed
		weights := make([]float64, config.TotalParts)
		for i := range weights {
			weights[i] = rand.ExpFloat64() / alpha
		}

		// Normalize weights
		sum := 0.0
		for _, w := range weights {
			sum += w
		}
		for i := range weights {
			weights[i] /= sum
		}

		// Calculate how many samples this participant gets
		samplesForThisParticipant := int(weights[config.PartIndex] * float64(classSize))

		// Ensure minimum samples if specified
		if config.MinSamples > 0 {
			minPerClass := config.MinSamples / len(classGroups)
			if samplesForThisParticipant < minPerClass {
				samplesForThisParticipant = minPerClass
			}
		}

		// Take samples for this participant
		if samplesForThisParticipant > classSize {
			samplesForThisParticipant = classSize
		}

		start := 0
		for i := 0; i < config.PartIndex; i++ {
			skip := int(weights[i] * float64(classSize))
			start += skip
		}

		end := start + samplesForThisParticipant
		if end > classSize {
			end = classSize
		}
		if start >= classSize {
			start = classSize - 1
			end = classSize
		}

		if start < end {
			partitionIndices = append(partitionIndices, indices[start:end]...)
		}
	}

	// Shuffle the final partition
	rand.Shuffle(len(partitionIndices), func(i, j int) {
		partitionIndices[i], partitionIndices[j] = partitionIndices[j], partitionIndices[i]
	})

	// Create partitioned data
	partFeatures := make([][]float64, len(partitionIndices))
	partLabels := make([]float64, len(partitionIndices))

	for i, idx := range partitionIndices {
		partFeatures[i] = features[idx]
		partLabels[i] = labels[idx]
	}

	return partFeatures, partLabels, nil
}

// labelSkewPartition creates partitions where each participant has only subset of classes
func (d *DataLoader) labelSkewPartition(features [][]float64, labels []float64, config *PartitionConfig) ([][]float64, []float64, error) {
	// Group samples by class
	classGroups := make(map[float64][]int)
	for i, label := range labels {
		classGroups[label] = append(classGroups[label], i)
	}

	classes := make([]float64, 0, len(classGroups))
	for class := range classGroups {
		classes = append(classes, class)
	}
	sort.Float64s(classes)

	// Each participant gets a subset of classes
	classesPerParticipant := len(classes) / config.TotalParts
	if classesPerParticipant == 0 {
		classesPerParticipant = 1
	}

	// Allow some overlap in classes if specified
	if config.OverlapRatio > 0 {
		overlap := int(float64(classesPerParticipant) * config.OverlapRatio)
		classesPerParticipant += overlap
	}

	startClass := config.PartIndex * (len(classes) / config.TotalParts)
	endClass := startClass + classesPerParticipant

	if endClass > len(classes) {
		endClass = len(classes)
	}
	if startClass >= len(classes) {
		startClass = len(classes) - 1
	}

	var partitionIndices []int

	// Collect all samples from assigned classes
	for i := startClass; i < endClass; i++ {
		class := classes[i]
		indices := classGroups[class]
		rand.Shuffle(len(indices), func(i, j int) { indices[i], indices[j] = indices[j], indices[i] })
		partitionIndices = append(partitionIndices, indices...)
	}

	// Shuffle the final partition
	rand.Shuffle(len(partitionIndices), func(i, j int) {
		partitionIndices[i], partitionIndices[j] = partitionIndices[j], partitionIndices[i]
	})

	// Create partitioned data
	partFeatures := make([][]float64, len(partitionIndices))
	partLabels := make([]float64, len(partitionIndices))

	for i, idx := range partitionIndices {
		partFeatures[i] = features[idx]
		partLabels[i] = labels[idx]
	}

	return partFeatures, partLabels, nil
}

func (d *DataLoader) parseCSV(r io.Reader) ([][]float64, []float64, error) {
	csvReader := csv.NewReader(r)

	// Skip header
	if _, err := csvReader.Read(); err != nil {
		return nil, nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	var features [][]float64
	var labels []float64
	labelMap := make(map[string]float64)
	nextLabelValue := 0.0

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read CSV record: %w", err)
		}

		// Last column is assumed to be the label
		featureVals := make([]float64, len(record)-1)
		for i := 0; i < len(record)-1; i++ {
			val, err := strconv.ParseFloat(record[i], 64)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse feature value: %w", err)
			}
			featureVals[i] = val
		}

		// Handle label - try to parse as float first, if that fails treat as categorical
		labelStr := record[len(record)-1]
		var label float64

		if labelValue, err := strconv.ParseFloat(labelStr, 64); err == nil {
			// It's already a numeric label
			label = labelValue
		} else {
			// It's a categorical label, convert to numeric
			if labelValue, exists := labelMap[labelStr]; exists {
				label = labelValue
			} else {
				// New categorical value, assign next available number
				labelMap[labelStr] = nextLabelValue
				label = nextLabelValue
				nextLabelValue++
			}
		}

		features = append(features, featureVals)
		labels = append(labels, label)
	}

	return features, labels, nil
}

func (d *DataLoader) parseJSON(r io.Reader) ([][]float64, []float64, error) {
	var data struct {
		Features [][]float64 `json:"features"`
		Labels   []float64   `json:"labels"`
	}

	if err := json.NewDecoder(r).Decode(&data); err != nil {
		return nil, nil, fmt.Errorf("failed to parse JSON data: %w", err)
	}

	if len(data.Features) != len(data.Labels) {
		return nil, nil, fmt.Errorf("mismatched features and labels length")
	}

	return data.Features, data.Labels, nil
}
