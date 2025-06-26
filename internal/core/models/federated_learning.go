package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type FLTrainingTask struct {
	SessionID   uuid.UUID        `json:"session_id"`
	RoundID     uuid.UUID        `json:"round_id"`
	RoundNumber int              `json:"round_number"`
	GlobalModel json.RawMessage  `json:"global_model"`
	Config      FLTrainingConfig `json:"config"`
	DataShard   *DataShard       `json:"data_shard,omitempty"`
}

type FLTrainingConfig struct {
	ModelType     string                 `json:"model_type"`
	LearningRate  float64                `json:"learning_rate"`
	BatchSize     int                    `json:"batch_size"`
	LocalEpochs   int                    `json:"local_epochs"`
	ModelConfig   map[string]interface{} `json:"model_config"`
	PrivacyConfig PrivacyConfig          `json:"privacy_config,omitempty"`
}

type PrivacyConfig struct {
	DifferentialPrivacy bool    `json:"differential_privacy"`
	NoiseMultiplier     float64 `json:"noise_multiplier,omitempty"`
	L2NormClip          float64 `json:"l2_norm_clip,omitempty"`
	SecureAggregation   bool    `json:"secure_aggregation"`
}

type DataShard struct {
	ID       string                 `json:"id"`
	Data     json.RawMessage        `json:"data"`
	Labels   json.RawMessage        `json:"labels,omitempty"`
	Size     int                    `json:"size"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type ModelUpdate struct {
	SessionID      uuid.UUID              `json:"session_id"`
	RoundID        uuid.UUID              `json:"round_id"`
	RunnerID       string                 `json:"runner_id"`
	Gradients      map[string][]float64   `json:"gradients"`
	Weights        map[string][]float64   `json:"weights,omitempty"`
	UpdateType     string                 `json:"update_type"`
	DataSize       int                    `json:"data_size"`
	Loss           float64                `json:"loss"`
	Accuracy       float64                `json:"accuracy,omitempty"`
	TrainingTime   time.Duration          `json:"training_time"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	PrivacyMetrics *PrivacyMetrics        `json:"privacy_metrics,omitempty"`
}

type PrivacyMetrics struct {
	NoiseScale      float64 `json:"noise_scale,omitempty"`
	ClippingApplied bool    `json:"clipping_applied,omitempty"`
	EpsilonUsed     float64 `json:"epsilon_used,omitempty"`
}

type TrainingMetrics struct {
	Loss            float64       `json:"loss"`
	Accuracy        float64       `json:"accuracy,omitempty"`
	TrainingTime    time.Duration `json:"training_time"`
	EpochsCompleted int           `json:"epochs_completed"`
	DataSamples     int           `json:"data_samples"`
	Convergence     float64       `json:"convergence,omitempty"`
}

type FLTaskResult struct {
	SessionID   uuid.UUID       `json:"session_id"`
	RoundID     uuid.UUID       `json:"round_id"`
	ModelUpdate ModelUpdate     `json:"model_update"`
	Metrics     TrainingMetrics `json:"metrics"`
	Status      string          `json:"status"`
	Error       string          `json:"error,omitempty"`
	CompletedAt time.Time       `json:"completed_at"`
}

func NewFLTaskResult(sessionID, roundID uuid.UUID, runnerID string) *FLTaskResult {
	return &FLTaskResult{
		SessionID:   sessionID,
		RoundID:     roundID,
		CompletedAt: time.Now(),
	}
}
