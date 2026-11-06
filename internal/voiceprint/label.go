package voiceprint

import "math"

const (
	// OwnerLabel is a turn that matches the enrolled voice.
	OwnerLabel = "owner"
	// OtherLabel is a turn that does not match the enrolled voice.
	OtherLabel = "other"
	// UnknownLabel is a turn whose similarity is between the owner and other
	// cuts, or a turn that was too short or non-finite to score.
	UnknownLabel = "unknown"
)

// LabelTurn maps a cosine similarity to owner, other, or unknown.
// A NaN or infinite score is unknown. The cuts are OwnerMinScore and
// OtherMaxScore.
func LabelTurn(score float64) string {
	if math.IsNaN(score) || math.IsInf(score, 0) {
		return UnknownLabel
	}
	if score >= OwnerMinScore {
		return OwnerLabel
	}
	if score <= OtherMaxScore {
		return OtherLabel
	}
	return UnknownLabel
}
