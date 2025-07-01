package training

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"
)

type RandomForestTrainer struct {
	config            *RandomForestConfig
	trees             []*DecisionTree
	features          [][]float64
	labels            []float64
	weights           map[string][]float64
	gradients         map[string][]float64
	oobError          float64
	featureImportance map[int]float64
	dataLoader        *DataLoader
	classWeights      map[float64]float64
	leafCount         int
	totalLeafCount    int
}

type RandomForestConfig struct {
	NumTrees               int         `json:"num_trees"`
	MaxDepth               int         `json:"max_depth"`
	MinSamplesSplit        int         `json:"min_samples_split"`
	MinSamplesLeaf         int         `json:"min_samples_leaf"`
	MaxFeatures            int         `json:"max_features"`
	Subsample              float64     `json:"subsample"`
	RandomState            int64       `json:"random_state"`
	BootstrapSamples       bool        `json:"bootstrap_samples"`
	OOBScore               bool        `json:"oob_score"`
	NumClasses             int         `json:"num_classes"`
	Criterion              string      `json:"criterion"`
	Splitter               string      `json:"splitter"`
	MaxFeaturesStrategy    string      `json:"max_features_strategy"`
	MinImpurityDecrease    float64     `json:"min_impurity_decrease"`
	BootstrapReplacement   bool        `json:"bootstrap_replacement"`
	ClassWeights           interface{} `json:"class_weights"`
	MaxLeafNodes           *int        `json:"max_leaf_nodes"`
	MinWeightFractionLeaf  float64     `json:"min_weight_fraction_leaf"`
	ParallelJobs           int         `json:"parallel_jobs"`
	WarmStart              bool        `json:"warm_start"`
	ValidationFraction     float64     `json:"validation_fraction"`
	NIterNoChange          *int        `json:"n_iter_no_change"`
	FeatureSelectionMethod string      `json:"feature_selection_method"`
	MissingValueStrategy   string      `json:"missing_value_strategy"`
}

type DecisionTree struct {
	Root       *TreeNode `json:"root"`
	MaxDepth   int       `json:"max_depth"`
	MinSplit   int       `json:"min_split"`
	MinLeaf    int       `json:"min_leaf"`
	MaxFeats   int       `json:"max_feats"`
	OOBIndices []int     `json:"oob_indices"`
}

type TreeNode struct {
	FeatureIndex int       `json:"feature_index"`
	Threshold    float64   `json:"threshold"`
	Left         *TreeNode `json:"left"`
	Right        *TreeNode `json:"right"`
	Value        float64   `json:"value"`
	IsLeaf       bool      `json:"is_leaf"`
	Samples      int       `json:"samples"`
	Impurity     float64   `json:"impurity"`
}

type SplitResult struct {
	FeatureIndex int
	Threshold    float64
	Impurity     float64
	LeftIndices  []int
	RightIndices []int
}

func NewRandomForestTrainer(config map[string]interface{}) (Trainer, error) {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var rfConfig RandomForestConfig
	if err := json.Unmarshal(configBytes, &rfConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal random forest config: %w", err)
	}

	// Set defaults for basic parameters
	if rfConfig.NumTrees <= 0 {
		rfConfig.NumTrees = 100
	}
	if rfConfig.MaxDepth <= 0 {
		rfConfig.MaxDepth = 10
	}
	if rfConfig.MinSamplesSplit <= 0 {
		rfConfig.MinSamplesSplit = 2
	}
	if rfConfig.MinSamplesLeaf <= 0 {
		rfConfig.MinSamplesLeaf = 1
	}
	if rfConfig.Subsample <= 0 {
		rfConfig.Subsample = 1.0
	}
	if rfConfig.RandomState == 0 {
		rfConfig.RandomState = time.Now().UnixNano()
	}

	// Set defaults for enhanced parameters
	if rfConfig.Criterion == "" {
		rfConfig.Criterion = "gini"
	}
	if rfConfig.Splitter == "" {
		rfConfig.Splitter = "best"
	}
	if rfConfig.MaxFeaturesStrategy == "" {
		rfConfig.MaxFeaturesStrategy = "sqrt"
	}
	if rfConfig.FeatureSelectionMethod == "" {
		rfConfig.FeatureSelectionMethod = "random"
	}
	if rfConfig.MissingValueStrategy == "" {
		rfConfig.MissingValueStrategy = "ignore"
	}
	if rfConfig.ParallelJobs == 0 {
		rfConfig.ParallelJobs = 1
	}

	// Default to traditional bootstrap with replacement
	if !rfConfig.BootstrapSamples {
		rfConfig.BootstrapReplacement = false
	} else if !rfConfig.BootstrapReplacement && rfConfig.BootstrapSamples {
		// If bootstrap is enabled but replacement not explicitly set, default to true
		rfConfig.BootstrapReplacement = true
	}

	trainer := &RandomForestTrainer{
		config:            &rfConfig,
		trees:             make([]*DecisionTree, 0, rfConfig.NumTrees),
		weights:           make(map[string][]float64),
		gradients:         make(map[string][]float64),
		featureImportance: make(map[int]float64),
		dataLoader:        NewDataLoader(""),
		classWeights:      make(map[float64]float64),
	}

	return trainer, nil
}

func (rf *RandomForestTrainer) LoadData(ctx context.Context, datasetCID string, format string) ([][]float64, []float64, error) {
	return rf.LoadPartitionedData(ctx, datasetCID, format, nil)
}

func (rf *RandomForestTrainer) LoadPartitionedData(ctx context.Context, datasetCID string, format string, partitionConfig *PartitionConfig) ([][]float64, []float64, error) {
	if datasetCID == "" {
		return nil, nil, fmt.Errorf("dataset CID is required")
	}

	features, labels, err := rf.dataLoader.LoadPartitionedData(ctx, datasetCID, format, partitionConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load data from IPFS: %w", err)
	}

	if len(features) == 0 {
		return nil, nil, fmt.Errorf("no data loaded from CID: %s", datasetCID)
	}

	if len(features) != len(labels) {
		return nil, nil, fmt.Errorf("feature and label count mismatch: %d features, %d labels", len(features), len(labels))
	}

	// Validate data dimensions
	if len(features[0]) == 0 {
		return nil, nil, fmt.Errorf("features have zero dimensions")
	}

	// Validate that all feature vectors have the same dimension
	featureDim := len(features[0])
	for i, feature := range features {
		if len(feature) != featureDim {
			return nil, nil, fmt.Errorf("inconsistent feature dimensions at sample %d: expected %d, got %d", i, featureDim, len(feature))
		}
	}

	// Validate labels are within reasonable range for classification
	uniqueLabels := make(map[float64]bool)
	for i, label := range labels {
		if math.IsNaN(label) || math.IsInf(label, 0) {
			return nil, nil, fmt.Errorf("invalid label at sample %d: %v", i, label)
		}
		uniqueLabels[label] = true
	}

	// Update config with actual number of classes if not set
	if rf.config.NumClasses == 0 {
		rf.config.NumClasses = len(uniqueLabels)
	}

	// Calculate class weights if specified
	rf.calculateClassWeights(labels, uniqueLabels)

	// Handle missing values if strategy is specified
	features = rf.handleMissingValues(features)

	return features, labels, nil
}

func (rf *RandomForestTrainer) Train(ctx context.Context, features [][]float64, labels []float64, epochs int, batchSize int, learningRate float64) ([]float64, float64, float64, error) {
	if len(features) == 0 || len(labels) == 0 {
		return nil, 0, 0, fmt.Errorf("empty training data")
	}

	rf.features = features
	rf.labels = labels

	numFeatures := len(features[0])
	if rf.config.MaxFeatures <= 0 || rf.config.MaxFeatures > numFeatures {
		rf.config.MaxFeatures = rf.calculateMaxFeatures(numFeatures)
	}

	rand.Seed(rf.config.RandomState)

	// Initialize trees - warm start keeps existing trees and adds more
	startTreeIndex := 0
	if rf.config.WarmStart && len(rf.trees) > 0 {
		// Keep existing trees and train additional ones
		startTreeIndex = len(rf.trees)
		if startTreeIndex >= rf.config.NumTrees {
			// Already have enough trees, no training needed
			accuracy := rf.calculateAccuracy(features, labels)
			loss := 1.0 - accuracy
			return rf.flattenWeights(), loss, accuracy, nil
		}
		// Extend trees slice to accommodate new trees
		newTrees := make([]*DecisionTree, rf.config.NumTrees)
		copy(newTrees, rf.trees)
		rf.trees = newTrees
	} else {
		// Fresh training or no existing trees
		rf.trees = make([]*DecisionTree, rf.config.NumTrees)
	}

	rf.totalLeafCount = 0
	oobPredictions := make([][]float64, len(features))
	oobCounts := make([]int, len(features))

	// Validation split for early stopping
	var validationFeatures [][]float64
	var validationLabels []float64
	var trainFeatures [][]float64
	var trainLabels []float64

	if rf.config.ValidationFraction > 0 && rf.config.NIterNoChange != nil {
		splitIndex := int(float64(len(features)) * (1.0 - rf.config.ValidationFraction))
		trainFeatures = features[:splitIndex]
		trainLabels = labels[:splitIndex]
		validationFeatures = features[splitIndex:]
		validationLabels = labels[splitIndex:]
	} else {
		trainFeatures = features
		trainLabels = labels
	}

	var bestValidationScore float64
	noImprovementCount := 0

	// Train trees with parallelization support
	if rf.config.ParallelJobs == 1 {
		// Sequential training
		for i := startTreeIndex; i < rf.config.NumTrees; i++ {
			if !rf.trainSingleTree(i, trainFeatures, trainLabels, oobPredictions, oobCounts) {
				break // Max leaf nodes reached
			}

			// Early stopping check
			if rf.config.NIterNoChange != nil && len(validationFeatures) > 0 {
				validationScore := rf.calculateValidationScore(validationFeatures, validationLabels)
				if i == startTreeIndex || validationScore > bestValidationScore {
					bestValidationScore = validationScore
					noImprovementCount = 0
				} else {
					noImprovementCount++
					if noImprovementCount >= *rf.config.NIterNoChange {
						// Trim trees to actual count
						rf.trees = rf.trees[:i+1]
						break
					}
				}
			}
		}
	} else {
		// Parallel training would be implemented here
		// For now, fall back to sequential
		for i := startTreeIndex; i < rf.config.NumTrees; i++ {
			if !rf.trainSingleTree(i, trainFeatures, trainLabels, oobPredictions, oobCounts) {
				break
			}
		}
	}
	// Calculate OOB error
	if rf.config.OOBScore {
		rf.oobError = rf.calculateOOBError(oobPredictions, oobCounts, labels)
	}

	// Calculate feature importance
	rf.calculateFeatureImportance()

	// Update weights and gradients for federated learning compatibility
	rf.updateWeightsAndGradients()

	// Calculate accuracy on training data
	accuracy := rf.calculateAccuracy(features, labels)
	loss := 1.0 - accuracy // Simple loss metric for classification

	return rf.flattenWeights(), loss, accuracy, nil
}

func (rf *RandomForestTrainer) trainSingleTree(treeIndex int, features [][]float64, labels []float64, oobPredictions [][]float64, oobCounts []int) bool {
	// Check max leaf nodes constraint
	if rf.config.MaxLeafNodes != nil && rf.totalLeafCount >= *rf.config.MaxLeafNodes {
		return false
	}

	// Bootstrap sampling
	bootstrapFeatures, bootstrapLabels, oobIndices := rf.bootstrap(features, labels)

	// Create and train tree
	tree := &DecisionTree{
		MaxDepth:   rf.config.MaxDepth,
		MinSplit:   rf.config.MinSamplesSplit,
		MinLeaf:    rf.config.MinSamplesLeaf,
		MaxFeats:   rf.config.MaxFeatures,
		OOBIndices: oobIndices,
	}

	rf.leafCount = 0 // Reset for this tree
	tree.Root = rf.buildTree(bootstrapFeatures, bootstrapLabels, rf.generateIndices(len(bootstrapFeatures)), 0)
	rf.trees[treeIndex] = tree
	rf.totalLeafCount += rf.leafCount

	// Calculate OOB predictions for this tree
	if rf.config.OOBScore && len(oobIndices) > 0 {
		for _, idx := range oobIndices {
			pred := rf.predictSample(tree, features[idx])
			if oobPredictions[idx] == nil {
				oobPredictions[idx] = make([]float64, rf.config.NumClasses)
			}
			if int(pred) < len(oobPredictions[idx]) {
				oobPredictions[idx][int(pred)]++
			}
			oobCounts[idx]++
		}
	}

	return true
}

func (rf *RandomForestTrainer) bootstrap(features [][]float64, labels []float64) ([][]float64, []float64, []int) {
	n := len(features)
	bootstrapSize := int(float64(n) * rf.config.Subsample)

	bootstrapFeatures := make([][]float64, bootstrapSize)
	bootstrapLabels := make([]float64, bootstrapSize)
	usedIndices := make(map[int]bool)
	oobIndices := make([]int, 0)

	if rf.config.BootstrapReplacement {
		// Bootstrap with replacement (traditional)
		for i := 0; i < bootstrapSize; i++ {
			idx := rand.Intn(n)
			usedIndices[idx] = true
			bootstrapFeatures[i] = make([]float64, len(features[idx]))
			copy(bootstrapFeatures[i], features[idx])
			bootstrapLabels[i] = labels[idx]
		}
	} else {
		// Bootstrap without replacement (subsampling)
		indices := make([]int, n)
		for i := range indices {
			indices[i] = i
		}
		rand.Shuffle(len(indices), func(i, j int) { indices[i], indices[j] = indices[j], indices[i] })

		for i := 0; i < bootstrapSize && i < n; i++ {
			idx := indices[i]
			usedIndices[idx] = true
			bootstrapFeatures[i] = make([]float64, len(features[idx]))
			copy(bootstrapFeatures[i], features[idx])
			bootstrapLabels[i] = labels[idx]
		}
	}

	// Collect out-of-bag indices
	for i := 0; i < n; i++ {
		if !usedIndices[i] {
			oobIndices = append(oobIndices, i)
		}
	}

	return bootstrapFeatures, bootstrapLabels, oobIndices
}

func (rf *RandomForestTrainer) buildTree(features [][]float64, labels []float64, indices []int, depth int) *TreeNode {
	if len(indices) < rf.config.MinSamplesSplit || depth >= rf.config.MaxDepth {
		rf.leafCount++
		return rf.createLeafNode(labels, indices)
	}

	// Check max leaf nodes constraint
	if rf.config.MaxLeafNodes != nil && rf.leafCount >= *rf.config.MaxLeafNodes {
		rf.leafCount++
		return rf.createLeafNode(labels, indices)
	}

	// Check minimum weight fraction constraint
	if rf.config.MinWeightFractionLeaf > 0 {
		totalWeight := rf.calculateTotalWeight(labels, indices)
		minWeight := rf.config.MinWeightFractionLeaf * totalWeight
		currentWeight := rf.calculateWeight(labels, indices)
		if currentWeight < minWeight {
			rf.leafCount++
			return rf.createLeafNode(labels, indices)
		}
	}

	bestSplit := rf.findBestSplit(features, labels, indices)
	if bestSplit == nil || len(bestSplit.LeftIndices) < rf.config.MinSamplesLeaf || len(bestSplit.RightIndices) < rf.config.MinSamplesLeaf {
		rf.leafCount++
		return rf.createLeafNode(labels, indices)
	}

	// Check minimum impurity decrease
	currentImpurity := rf.calculateImpurity(labels, indices)
	impurityDecrease := currentImpurity - bestSplit.Impurity
	if impurityDecrease < rf.config.MinImpurityDecrease {
		rf.leafCount++
		return rf.createLeafNode(labels, indices)
	}

	node := &TreeNode{
		FeatureIndex: bestSplit.FeatureIndex,
		Threshold:    bestSplit.Threshold,
		IsLeaf:       false,
		Samples:      len(indices),
		Impurity:     bestSplit.Impurity,
	}

	node.Left = rf.buildTree(features, labels, bestSplit.LeftIndices, depth+1)
	node.Right = rf.buildTree(features, labels, bestSplit.RightIndices, depth+1)

	return node
}

func (rf *RandomForestTrainer) findBestSplit(features [][]float64, labels []float64, indices []int) *SplitResult {
	if len(indices) == 0 {
		return nil
	}

	numFeatures := len(features[0])
	featureIndices := rf.selectFeatures(numFeatures, indices, features, labels)

	var bestSplit *SplitResult
	bestImpurity := math.Inf(1)

	if strings.ToLower(rf.config.Splitter) == "random" {
		// Random split for Extra Trees
		if len(featureIndices) == 0 {
			return nil
		}
		featureIdx := featureIndices[rand.Intn(len(featureIndices))]

		// Get feature values for random threshold selection
		featureValues := make([]float64, len(indices))
		for i, idx := range indices {
			featureValues[i] = features[idx][featureIdx]
		}
		sort.Float64s(featureValues)

		// Random threshold between min and max values
		if len(featureValues) > 1 {
			minVal := featureValues[0]
			maxVal := featureValues[len(featureValues)-1]
			if minVal != maxVal {
				threshold := minVal + rand.Float64()*(maxVal-minVal)

				leftIndices, rightIndices := rf.splitIndices(features, indices, featureIdx, threshold)
				if len(leftIndices) > 0 && len(rightIndices) > 0 {
					impurity := rf.calculateWeightedImpurity(labels, leftIndices, rightIndices)
					bestSplit = &SplitResult{
						FeatureIndex: featureIdx,
						Threshold:    threshold,
						Impurity:     impurity,
						LeftIndices:  leftIndices,
						RightIndices: rightIndices,
					}
				}
			}
		}
	} else {
		// Best split (traditional Random Forest)
		for _, featureIdx := range featureIndices {
			values := make([]float64, len(indices))
			for i, idx := range indices {
				values[i] = features[idx][featureIdx]
			}

			sort.Float64s(values)

			for i := 0; i < len(values)-1; i++ {
				if values[i] == values[i+1] {
					continue
				}

				threshold := (values[i] + values[i+1]) / 2
				leftIndices, rightIndices := rf.splitIndices(features, indices, featureIdx, threshold)

				if len(leftIndices) == 0 || len(rightIndices) == 0 {
					continue
				}

				impurity := rf.calculateWeightedImpurity(labels, leftIndices, rightIndices)

				if impurity < bestImpurity {
					bestImpurity = impurity
					bestSplit = &SplitResult{
						FeatureIndex: featureIdx,
						Threshold:    threshold,
						Impurity:     impurity,
						LeftIndices:  leftIndices,
						RightIndices: rightIndices,
					}
				}
			}
		}
	}

	return bestSplit
}

func (rf *RandomForestTrainer) selectFeatures(numFeatures int, indices []int, features [][]float64, labels []float64) []int {
	switch strings.ToLower(rf.config.FeatureSelectionMethod) {
	case "importance":
		return rf.selectFeaturesByImportance(numFeatures)
	case "correlation":
		return rf.selectFeaturesByCorrelation(numFeatures, indices, features, labels)
	case "random":
		fallthrough
	default:
		return rf.selectRandomFeatures(numFeatures)
	}
}

func (rf *RandomForestTrainer) selectRandomFeatures(numFeatures int) []int {
	indices := make([]int, numFeatures)
	for i := range indices {
		indices[i] = i
	}

	rand.Shuffle(len(indices), func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})

	maxFeatures := rf.config.MaxFeatures
	if maxFeatures > numFeatures {
		maxFeatures = numFeatures
	}

	return indices[:maxFeatures]
}

func (rf *RandomForestTrainer) selectFeaturesByImportance(numFeatures int) []int {
	if len(rf.featureImportance) == 0 {
		return rf.selectRandomFeatures(numFeatures)
	}

	type featureScore struct {
		index      int
		importance float64
	}

	scores := make([]featureScore, 0, numFeatures)
	for i := 0; i < numFeatures; i++ {
		importance := rf.featureImportance[i]
		scores = append(scores, featureScore{i, importance})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].importance > scores[j].importance
	})

	maxFeatures := rf.config.MaxFeatures
	if maxFeatures > len(scores) {
		maxFeatures = len(scores)
	}

	result := make([]int, maxFeatures)
	for i := 0; i < maxFeatures; i++ {
		result[i] = scores[i].index
	}

	return result
}

func (rf *RandomForestTrainer) selectFeaturesByCorrelation(numFeatures int, indices []int, features [][]float64, labels []float64) []int {
	// Simplified correlation-based selection - in practice this would be more sophisticated
	correlations := make([]float64, numFeatures)

	for f := 0; f < numFeatures; f++ {
		var sumX, sumY, sumXY, sumX2, sumY2 float64
		n := float64(len(indices))

		for _, idx := range indices {
			x := features[idx][f]
			y := labels[idx]
			sumX += x
			sumY += y
			sumXY += x * y
			sumX2 += x * x
			sumY2 += y * y
		}

		numerator := n*sumXY - sumX*sumY
		denominator := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))

		if denominator != 0 {
			correlations[f] = math.Abs(numerator / denominator)
		}
	}

	type featureScore struct {
		index       int
		correlation float64
	}

	scores := make([]featureScore, numFeatures)
	for i := 0; i < numFeatures; i++ {
		scores[i] = featureScore{i, correlations[i]}
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].correlation > scores[j].correlation
	})

	maxFeatures := rf.config.MaxFeatures
	if maxFeatures > numFeatures {
		maxFeatures = numFeatures
	}

	result := make([]int, maxFeatures)
	for i := 0; i < maxFeatures; i++ {
		result[i] = scores[i].index
	}

	return result
}

func (rf *RandomForestTrainer) splitIndices(features [][]float64, indices []int, featureIdx int, threshold float64) ([]int, []int) {
	var leftIndices, rightIndices []int

	for _, idx := range indices {
		if features[idx][featureIdx] <= threshold {
			leftIndices = append(leftIndices, idx)
		} else {
			rightIndices = append(rightIndices, idx)
		}
	}

	return leftIndices, rightIndices
}

func (rf *RandomForestTrainer) calculateWeightedImpurity(labels []float64, leftIndices, rightIndices []int) float64 {
	totalSamples := len(leftIndices) + len(rightIndices)
	if totalSamples == 0 {
		return 0
	}

	leftWeight := float64(len(leftIndices)) / float64(totalSamples)
	rightWeight := float64(len(rightIndices)) / float64(totalSamples)

	leftImpurity := rf.calculateImpurity(labels, leftIndices)
	rightImpurity := rf.calculateImpurity(labels, rightIndices)

	return leftWeight*leftImpurity + rightWeight*rightImpurity
}

func (rf *RandomForestTrainer) calculateMaxFeatures(numFeatures int) int {
	switch strings.ToLower(rf.config.MaxFeaturesStrategy) {
	case "sqrt":
		return int(math.Sqrt(float64(numFeatures)))
	case "log2":
		return int(math.Log2(float64(numFeatures)))
	case "auto":
		return int(math.Sqrt(float64(numFeatures)))
	case "all", "none":
		return numFeatures
	default:
		// If it's not a recognized string, treat as sqrt
		return int(math.Sqrt(float64(numFeatures)))
	}
}

func (rf *RandomForestTrainer) calculateImpurity(labels []float64, indices []int) float64 {
	switch strings.ToLower(rf.config.Criterion) {
	case "entropy":
		return rf.calculateEntropyImpurity(labels, indices)
	case "gini":
		fallthrough
	default:
		return rf.calculateGiniImpurity(labels, indices)
	}
}

func (rf *RandomForestTrainer) calculateGiniImpurity(labels []float64, indices []int) float64 {
	if len(indices) == 0 {
		return 0
	}

	classCounts := make(map[float64]int)
	for _, idx := range indices {
		classCounts[labels[idx]]++
	}

	impurity := 1.0
	totalSamples := len(indices)

	for _, count := range classCounts {
		probability := float64(count) / float64(totalSamples)
		impurity -= probability * probability
	}

	return impurity
}

func (rf *RandomForestTrainer) calculateEntropyImpurity(labels []float64, indices []int) float64 {
	if len(indices) == 0 {
		return 0
	}

	classCounts := make(map[float64]int)
	for _, idx := range indices {
		classCounts[labels[idx]]++
	}

	entropy := 0.0
	totalSamples := len(indices)

	for _, count := range classCounts {
		if count > 0 {
			probability := float64(count) / float64(totalSamples)
			entropy -= probability * math.Log2(probability)
		}
	}

	return entropy
}

func (rf *RandomForestTrainer) calculateClassWeights(labels []float64, uniqueLabels map[float64]bool) {
	if rf.config.ClassWeights == nil {
		return
	}

	switch weights := rf.config.ClassWeights.(type) {
	case string:
		if weights == "balanced" {
			// Calculate balanced class weights
			totalSamples := float64(len(labels))
			numClasses := float64(len(uniqueLabels))

			classCounts := make(map[float64]int)
			for _, label := range labels {
				classCounts[label]++
			}

			for class := range uniqueLabels {
				count := float64(classCounts[class])
				if count > 0 {
					rf.classWeights[class] = totalSamples / (numClasses * count)
				} else {
					rf.classWeights[class] = 1.0
				}
			}
		}
	case map[string]interface{}:
		// Custom class weights
		for classStr, weightInterface := range weights {
			if classVal, err := strconv.ParseFloat(classStr, 64); err == nil {
				if weight, ok := weightInterface.(float64); ok {
					rf.classWeights[classVal] = weight
				}
			}
		}
	default:
		// Default: equal weights
		for class := range uniqueLabels {
			rf.classWeights[class] = 1.0
		}
	}
}

func (rf *RandomForestTrainer) handleMissingValues(features [][]float64) [][]float64 {
	switch strings.ToLower(rf.config.MissingValueStrategy) {
	case "impute_mean":
		return rf.imputeMean(features)
	case "impute_mode":
		return rf.imputeMode(features)
	case "ignore":
		fallthrough
	default:
		return features
	}
}

func (rf *RandomForestTrainer) imputeMean(features [][]float64) [][]float64 {
	if len(features) == 0 {
		return features
	}

	numFeatures := len(features[0])
	means := make([]float64, numFeatures)
	counts := make([]int, numFeatures)

	// Calculate means ignoring NaN values
	for _, sample := range features {
		for j, value := range sample {
			if !math.IsNaN(value) {
				means[j] += value
				counts[j]++
			}
		}
	}

	for j := range means {
		if counts[j] > 0 {
			means[j] /= float64(counts[j])
		}
	}

	// Replace NaN values with means
	result := make([][]float64, len(features))
	for i, sample := range features {
		result[i] = make([]float64, len(sample))
		for j, value := range sample {
			if math.IsNaN(value) {
				result[i][j] = means[j]
			} else {
				result[i][j] = value
			}
		}
	}

	return result
}

func (rf *RandomForestTrainer) imputeMode(features [][]float64) [][]float64 {
	if len(features) == 0 {
		return features
	}

	numFeatures := len(features[0])
	modes := make([]float64, numFeatures)

	// Calculate mode for each feature
	for j := 0; j < numFeatures; j++ {
		valueCounts := make(map[float64]int)
		for _, sample := range features {
			if !math.IsNaN(sample[j]) {
				valueCounts[sample[j]]++
			}
		}

		maxCount := 0
		for value, count := range valueCounts {
			if count > maxCount {
				maxCount = count
				modes[j] = value
			}
		}
	}

	// Replace NaN values with modes
	result := make([][]float64, len(features))
	for i, sample := range features {
		result[i] = make([]float64, len(sample))
		for j, value := range sample {
			if math.IsNaN(value) {
				result[i][j] = modes[j]
			} else {
				result[i][j] = value
			}
		}
	}

	return result
}

func (rf *RandomForestTrainer) calculateValidationScore(features [][]float64, labels []float64) float64 {
	return rf.calculateAccuracy(features, labels)
}

func (rf *RandomForestTrainer) calculateTotalWeight(labels []float64, indices []int) float64 {
	totalWeight := 0.0
	for _, idx := range indices {
		label := labels[idx]
		weight := rf.classWeights[label]
		if weight == 0 {
			weight = 1.0
		}
		totalWeight += weight
	}
	return totalWeight
}

func (rf *RandomForestTrainer) calculateWeight(labels []float64, indices []int) float64 {
	return rf.calculateTotalWeight(labels, indices)
}

func (rf *RandomForestTrainer) createLeafNode(labels []float64, indices []int) *TreeNode {
	classCounts := make(map[float64]int)
	for _, idx := range indices {
		classCounts[labels[idx]]++
	}

	var majorityClass float64
	maxCount := 0
	for class, count := range classCounts {
		if count > maxCount {
			maxCount = count
			majorityClass = class
		}
	}

	return &TreeNode{
		Value:    majorityClass,
		IsLeaf:   true,
		Samples:  len(indices),
		Impurity: rf.calculateImpurity(labels, indices),
	}
}

func (rf *RandomForestTrainer) generateIndices(length int) []int {
	indices := make([]int, length)
	for i := range indices {
		indices[i] = i
	}
	return indices
}

func (rf *RandomForestTrainer) predictSample(tree *DecisionTree, sample []float64) float64 {
	node := tree.Root
	for !node.IsLeaf {
		if sample[node.FeatureIndex] <= node.Threshold {
			node = node.Left
		} else {
			node = node.Right
		}
	}
	return node.Value
}

func (rf *RandomForestTrainer) calculateOOBError(oobPredictions [][]float64, oobCounts []int, labels []float64) float64 {
	correct := 0
	total := 0

	for i, predictions := range oobPredictions {
		if oobCounts[i] > 0 {
			maxIdx := 0
			maxCount := predictions[0]
			for j, count := range predictions {
				if count > maxCount {
					maxCount = count
					maxIdx = j
				}
			}

			if float64(maxIdx) == labels[i] {
				correct++
			}
			total++
		}
	}

	if total == 0 {
		return 0
	}
	return 1.0 - float64(correct)/float64(total)
}

func (rf *RandomForestTrainer) calculateFeatureImportance() {
	totalImportance := make(map[int]float64)

	for _, tree := range rf.trees {
		treeImportance := rf.calculateTreeFeatureImportance(tree.Root)
		for feature, importance := range treeImportance {
			totalImportance[feature] += importance
		}
	}

	// Normalize by number of trees
	for feature, importance := range totalImportance {
		rf.featureImportance[feature] = importance / float64(len(rf.trees))
	}
}

func (rf *RandomForestTrainer) calculateTreeFeatureImportance(node *TreeNode) map[int]float64 {
	importance := make(map[int]float64)
	if node == nil || node.IsLeaf {
		return importance
	}

	weightedSamples := float64(node.Samples)
	leftSamples := float64(node.Left.Samples)
	rightSamples := float64(node.Right.Samples)

	impurityDecrease := node.Impurity - (leftSamples/weightedSamples)*node.Left.Impurity - (rightSamples/weightedSamples)*node.Right.Impurity
	importance[node.FeatureIndex] = weightedSamples * impurityDecrease

	leftImportance := rf.calculateTreeFeatureImportance(node.Left)
	rightImportance := rf.calculateTreeFeatureImportance(node.Right)

	for feature, imp := range leftImportance {
		importance[feature] += imp
	}
	for feature, imp := range rightImportance {
		importance[feature] += imp
	}

	return importance
}

func (rf *RandomForestTrainer) updateWeightsAndGradients() {
	// Convert trees to weights for federated learning compatibility
	treeWeights := make([]float64, 0)
	for _, tree := range rf.trees {
		treeData := rf.serializeTree(tree)
		treeWeights = append(treeWeights, treeData...)
	}

	rf.weights["trees"] = treeWeights
	rf.gradients["trees"] = make([]float64, len(treeWeights)) // Gradients not directly applicable to RF

	// Store feature importance as weights
	featureImportanceSlice := make([]float64, 0)
	for i := 0; i < len(rf.featureImportance); i++ {
		featureImportanceSlice = append(featureImportanceSlice, rf.featureImportance[i])
	}
	rf.weights["feature_importance"] = featureImportanceSlice
}

func (rf *RandomForestTrainer) serializeTree(tree *DecisionTree) []float64 {
	// Simple serialization - in practice this would be more sophisticated
	data := []float64{
		float64(tree.MaxDepth),
		float64(tree.MinSplit),
		float64(tree.MinLeaf),
		float64(tree.MaxFeats),
	}

	nodeData := rf.serializeNode(tree.Root)
	data = append(data, nodeData...)

	return data
}

func (rf *RandomForestTrainer) serializeNode(node *TreeNode) []float64 {
	if node == nil {
		return []float64{-1} // Null marker
	}

	data := []float64{
		float64(node.FeatureIndex),
		node.Threshold,
		node.Value,
		float64(node.Samples),
		node.Impurity,
	}

	if node.IsLeaf {
		data = append(data, 1)
	} else {
		data = append(data, 0)
		data = append(data, rf.serializeNode(node.Left)...)
		data = append(data, rf.serializeNode(node.Right)...)
	}

	return data
}

func (rf *RandomForestTrainer) calculateAccuracy(features [][]float64, labels []float64) float64 {
	correct := 0
	total := len(features)

	for i, sample := range features {
		prediction := rf.Predict(sample)
		if prediction == labels[i] {
			correct++
		}
	}

	return float64(correct) / float64(total)
}

func (rf *RandomForestTrainer) Predict(sample []float64) float64 {
	if len(rf.trees) == 0 {
		return 0
	}

	classCounts := make(map[float64]int)
	for _, tree := range rf.trees {
		prediction := rf.predictSample(tree, sample)
		classCounts[prediction]++
	}

	var majorityClass float64
	maxCount := 0
	for class, count := range classCounts {
		if count > maxCount {
			maxCount = count
			majorityClass = class
		}
	}

	return majorityClass
}

func (rf *RandomForestTrainer) flattenWeights() []float64 {
	var flattened []float64
	for _, weights := range rf.weights {
		flattened = append(flattened, weights...)
	}
	return flattened
}

func (rf *RandomForestTrainer) GetModelWeights() map[string][]float64 {
	return rf.weights
}

func (rf *RandomForestTrainer) GetGradients() map[string][]float64 {
	return rf.gradients
}

func (rf *RandomForestTrainer) GetFeatureImportance() map[int]float64 {
	return rf.featureImportance
}

func (rf *RandomForestTrainer) GetOOBError() float64 {
	return rf.oobError
}

func (rf *RandomForestTrainer) GetTrees() []*DecisionTree {
	return rf.trees
}
