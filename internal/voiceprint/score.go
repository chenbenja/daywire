package voiceprint

import (
	"fmt"
	"math"
)

// OwnerMinScore is the cosine similarity that labels a turn as the enrolled
// owner. On the LibriSpeech fixtures with NeMo SpeakerNet, the owner turn
// scores about 0.73.
const OwnerMinScore = 0.45

// OtherMaxScore is the cosine similarity at or below which a turn is other.
// The same fixtures score the other reader about 0.24. Scores strictly
// between OtherMaxScore and OwnerMinScore are unknown.
const OtherMaxScore = 0.30

// Cosine returns the cosine similarity of two embeddings.
func Cosine(a, b []float32) (float64, error) {
	if err := Check(a); err != nil {
		return 0, err
	}
	if len(a) != len(b) {
		return 0, fmt.Errorf("embedding length %d != %d", len(a), len(b))
	}
	if err := Check(b); err != nil {
		return 0, err
	}
	var dot, na, nb float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0, fmt.Errorf("embedding norm is zero")
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb)), nil
}

// MeanEmbedding returns the unweighted mean of same-length embeddings.
func MeanEmbedding(vectors [][]float32) ([]float32, error) {
	if len(vectors) == 0 {
		return nil, fmt.Errorf("no embeddings to average")
	}
	dim := len(vectors[0])
	if err := Check(vectors[0]); err != nil {
		return nil, err
	}
	acc := make([]float64, dim)
	for _, v := range vectors {
		if len(v) != dim {
			return nil, fmt.Errorf("embedding length %d != %d", len(v), dim)
		}
		if err := Check(v); err != nil {
			return nil, err
		}
		for i, x := range v {
			acc[i] += float64(x)
		}
	}
	out := make([]float32, dim)
	n := float64(len(vectors))
	for i := range out {
		out[i] = float32(acc[i] / n)
	}
	return out, nil
}

// Check reports whether v is a finite embedding the profile file can store.
func Check(v []float32) error {
	if len(v) == 0 || len(v) > 4096 {
		return fmt.Errorf("embedding length %d is invalid", len(v))
	}
	for _, x := range v {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return fmt.Errorf("embedding is not finite")
		}
	}
	return nil
}
