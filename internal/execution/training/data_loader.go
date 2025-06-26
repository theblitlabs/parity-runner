package training

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// DataLoader handles loading training data from IPFS/Filecoin
type DataLoader struct {
	ipfsGateway string
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

	switch strings.ToLower(format) {
	case "csv":
		return d.parseCSV(resp.Body)
	case "json":
		return d.parseJSON(resp.Body)
	default:
		return nil, nil, fmt.Errorf("unsupported data format: %s", format)
	}
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
