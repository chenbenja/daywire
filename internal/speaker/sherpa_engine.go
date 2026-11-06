//go:build cgo && speaker

package speaker

import (
	"context"
	"fmt"
	"log/slog"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/v2/sherpa_onnx"
)

const sherpaAvailable = true

// clusterThreshold is sherpa's cosine-distance cut when the speaker count is
// unknown. Distance is 1-cosine, so 0.5 keeps speakers whose embeddings agree
// above cosine 0.5 in one cluster.
const clusterThreshold = 0.5

type sherpaEngine struct {
	ex  *sherpa.SpeakerEmbeddingExtractor
	sd  *sherpa.OfflineSpeakerDiarization
	log *slog.Logger
}

func newSherpaEngine(models Models, logger *slog.Logger) (Engine, error) {
	ex := sherpa.NewSpeakerEmbeddingExtractor(&sherpa.SpeakerEmbeddingExtractorConfig{
		Model:      models.Embedding,
		NumThreads: 2,
		Provider:   "cpu",
	})
	if ex == nil {
		return nil, fmt.Errorf("create speaker embedding extractor")
	}
	sd := sherpa.NewOfflineSpeakerDiarization(&sherpa.OfflineSpeakerDiarizationConfig{
		Segmentation: sherpa.OfflineSpeakerSegmentationModelConfig{
			Pyannote: sherpa.OfflineSpeakerSegmentationPyannoteModelConfig{
				Model:            models.Segmentation,
				WindowShiftRatio: 0.1,
			},
			NumThreads: 2,
			Provider:   "cpu",
		},
		Embedding: sherpa.SpeakerEmbeddingExtractorConfig{
			Model:      models.Embedding,
			NumThreads: 2,
			Provider:   "cpu",
		},
		Clustering: sherpa.FastClusteringConfig{
			Threshold: clusterThreshold,
		},
		MinDurationOn:  0.2,
		MinDurationOff: 0.2,
	})
	if sd == nil {
		sherpa.DeleteSpeakerEmbeddingExtractor(ex)
		return nil, fmt.Errorf("create speaker diarization")
	}
	logger.Info("speaker engine ready", "dim", ex.Dim())
	return &sherpaEngine{ex: ex, sd: sd, log: logger}, nil
}

func (e *sherpaEngine) Close() error {
	if e.sd != nil {
		sherpa.DeleteOfflineSpeakerDiarization(e.sd)
		e.sd = nil
	}
	if e.ex != nil {
		sherpa.DeleteSpeakerEmbeddingExtractor(e.ex)
		e.ex = nil
	}
	return nil
}

func (e *sherpaEngine) Embed(ctx context.Context, samples []float32, sampleRate int) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sampleRate != SampleRate {
		return nil, fmt.Errorf("sample rate %d, want %d", sampleRate, SampleRate)
	}
	if len(samples) < SampleRate/2 {
		return nil, fmt.Errorf("clip is shorter than 0.5s")
	}
	if e.sd.SampleRate() != SampleRate {
		return nil, fmt.Errorf("diarization model sample rate is %d", e.sd.SampleRate())
	}
	stream := e.ex.CreateStream()
	defer sherpa.DeleteOnlineStream(stream)
	stream.AcceptWaveform(sampleRate, samples)
	stream.InputFinished()
	if !e.ex.IsReady(stream) {
		return nil, fmt.Errorf("speaker embedding is not ready")
	}
	vec := e.ex.Compute(stream)
	if err := checkEmbedding(vec); err != nil {
		return nil, err
	}
	return vec, nil
}

func (e *sherpaEngine) Diarize(ctx context.Context, samples []float32, sampleRate int, speakers int) ([]Segment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if speakers < 0 {
		return nil, fmt.Errorf("speaker count is negative")
	}
	if sampleRate != e.sd.SampleRate() {
		return nil, fmt.Errorf("sample rate %d, want %d", sampleRate, e.sd.SampleRate())
	}
	if len(samples) < SampleRate {
		return nil, fmt.Errorf("clip is shorter than 1s")
	}
	cfg := sherpa.OfflineSpeakerDiarizationConfig{
		Clustering: sherpa.FastClusteringConfig{Threshold: clusterThreshold},
	}
	if speakers > 0 {
		cfg.Clustering.NumClusters = speakers
	}
	e.sd.SetConfig(&cfg)
	raw := e.sd.Process(samples)
	out := make([]Segment, 0, len(raw))
	for _, seg := range raw {
		out = append(out, Segment{
			Speaker: seg.Speaker,
			Start:   float64(seg.Start),
			End:     float64(seg.End),
		})
	}
	e.log.DebugContext(ctx, "diarized", "segments", len(out), "speakers", speakers)
	return out, nil
}
