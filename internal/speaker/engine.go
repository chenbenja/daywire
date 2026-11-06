package speaker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// ErrEngineUnavailable means this build cannot run local speaker models.
var ErrEngineUnavailable = errors.New("speaker engine unavailable")

// Segment is one diarized turn. Times are seconds from the start of the clip.
type Segment struct {
	Speaker int
	Start   float64
	End     float64
}

// Engine extracts embeddings and diarizes clips on this machine.
type Engine interface {
	Embed(ctx context.Context, samples []float32, sampleRate int) ([]float32, error)
	// Diarize splits samples into speaker turns. speakers is the known count.
	// Zero asks the model to choose a count from cosine distance.
	Diarize(ctx context.Context, samples []float32, sampleRate int, speakers int) ([]Segment, error)
	Close() error
}

// NewEngine downloads the pinned models when they are missing and loads them.
// modelDir empty uses DefaultModelDir. The engine runs on CPU. It does not
// open a socket.
func NewEngine(ctx context.Context, modelDir string, logger *slog.Logger) (Engine, error) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !sherpaAvailable {
		return nil, fmt.Errorf("rebuild with cgo to link sherpa-onnx: %w", ErrEngineUnavailable)
	}
	if modelDir == "" {
		var err error
		modelDir, err = DefaultModelDir()
		if err != nil {
			return nil, err
		}
	}
	models, err := EnsureModels(ctx, modelDir, logger)
	if err != nil {
		return nil, err
	}
	return newSherpaEngine(models, logger)
}

// DefaultModelDir is the checksummed model cache.
//
// From a Daywire checkout it is <repo>/output/speaker-models, which is
// gitignored. Otherwise it is $XDG_CACHE_HOME/daywire/speaker or
// ~/.cache/daywire/speaker. DAYWIRE_SPEAKER_MODEL_DIR overrides both.
func DefaultModelDir() (string, error) {
	if v := os.Getenv("DAYWIRE_SPEAKER_MODEL_DIR"); v != "" {
		return v, nil
	}
	if root, err := moduleRoot(); err == nil {
		return filepath.Join(root, "output", "speaker-models"), nil
	}
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("model cache dir: %w", err)
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "daywire", "speaker"), nil
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}
