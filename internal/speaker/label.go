package speaker

import "errors"

const (
	// OwnerLabel is the cluster that matches the enrolled voice.
	OwnerLabel = "owner"
	// OtherLabel is every cluster that does not match.
	OtherLabel = "other"
)

// ErrNoOwner means no cluster reached the owner score.
var ErrNoOwner = errors.New("no cluster matched the enrolled speaker")

// ErrAmbiguousOwner means two clusters tied for the best score.
var ErrAmbiguousOwner = errors.New("owner match is ambiguous")

// ClusterScore is one diarized speaker and its similarity to the enrollment.
type ClusterScore struct {
	Speaker int
	Score   float64
}

// Assignment is the owner decision for a set of clusters.
type Assignment struct {
	// Owner is the diarization speaker id labeled owner.
	Owner  int
	Labels map[int]string
}

// AssignOwner labels the highest score at or above min as owner.
// Every other cluster is other. A tie for the best score is an error.
func AssignOwner(scores []ClusterScore, min float64) (Assignment, error) {
	if len(scores) == 0 {
		return Assignment{}, ErrNoOwner
	}
	best := scores[0].Score
	bestN := 1
	for _, s := range scores[1:] {
		switch {
		case s.Score > best:
			best = s.Score
			bestN = 1
		case s.Score == best:
			bestN++
		}
	}
	if best < min {
		return Assignment{}, ErrNoOwner
	}
	if bestN != 1 {
		return Assignment{}, ErrAmbiguousOwner
	}
	labels := make(map[int]string, len(scores))
	owner := 0
	for _, s := range scores {
		if s.Score == best {
			labels[s.Speaker] = OwnerLabel
			owner = s.Speaker
			continue
		}
		labels[s.Speaker] = OtherLabel
	}
	return Assignment{Owner: owner, Labels: labels}, nil
}
