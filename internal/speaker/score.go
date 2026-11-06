package speaker

import (
	"fmt"
	"math"
)

// DefaultOwnerMinScore is the cosine similarity that labels a cluster as the
// enrolled owner. On the public English fixtures with NeMo speakernet, the
// owner cluster scores about 0.76 and the other speaker about 0.18.
const DefaultOwnerMinScore = 0.45

// Cosine returns the cosine similarity of two embeddings.
func Cosine(a, b []float32) (float64, error) {
	if err := checkEmbedding(a); err != nil {
		return 0, err
	}
	if len(a) != len(b) {
		return 0, fmt.Errorf("embedding length %d != %d", len(a), len(b))
	}
	if err := checkEmbedding(b); err != nil {
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
	if err := checkEmbedding(vectors[0]); err != nil {
		return nil, err
	}
	acc := make([]float64, dim)
	for _, v := range vectors {
		if len(v) != dim {
			return nil, fmt.Errorf("embedding length %d != %d", len(v), dim)
		}
		if err := checkEmbedding(v); err != nil {
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

func checkEmbedding(v []float32) error {
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
