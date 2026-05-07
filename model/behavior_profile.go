package model

import "time"

// BehaviorDNA is the computed behavioral profile for a user.
type BehaviorDNA struct {
	DominantCategory    string   `json:"dominant_category"`
	ImpulseFrequency    int      `json:"impulse_frequency"`     // count in last 30 days
	LuxuryDriftIndex    float64  `json:"luxury_drift_index"`    // % change in discretionary spend
	LuxuryDriftDetected bool     `json:"luxury_drift_detected"` // true if drift > 20%
	TopBrands           []string `json:"top_brands"`            // top 3 by frequency
	InsufficientData    bool     `json:"insufficient_data"`     // true if < 5 transactions
}

// BehaviorProfile is the GORM model persisted to behavior_profiles table.
type BehaviorProfile struct {
	ID           uint      `json:"id"            gorm:"primaryKey;autoIncrement"`
	UserID       string    `json:"user_id"       gorm:"index"`
	ComputedDate time.Time `json:"computed_date"`
	// BehaviorDNA fields flattened for easy querying
	DominantCategory    string    `json:"dominant_category"`
	ImpulseFrequency    int       `json:"impulse_frequency"`
	LuxuryDriftIndex    float64   `json:"luxury_drift_index"`
	LuxuryDriftDetected bool      `json:"luxury_drift_detected"`
	TopBrands           string    `json:"top_brands"` // JSON-encoded []string
	InsufficientData    bool      `json:"insufficient_data"`
	CreatedAt           time.Time `json:"created_at"`
}
