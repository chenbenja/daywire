package speaker

import "github.com/chenbenja/daywire/internal/voiceprint"

// DefaultOwnerMinScore is the cosine similarity that labels a cluster as the
// enrolled owner. On the LibriSpeech fixtures with NeMo SpeakerNet, the owner
// cluster scores about 0.73 and the other reader about 0.24. The phone path
// also keeps an unknown band under this cut; see voiceprint.LabelTurn.
const DefaultOwnerMinScore = voiceprint.OwnerMinScore

// Cosine returns the cosine similarity of two embeddings.
func Cosine(a, b []float32) (float64, error) {
	return voiceprint.Cosine(a, b)
}

// MeanEmbedding returns the unweighted mean of same-length embeddings.
func MeanEmbedding(vectors [][]float32) ([]float32, error) {
	return voiceprint.MeanEmbedding(vectors)
}

func checkEmbedding(v []float32) error {
	return voiceprint.Check(v)
}
