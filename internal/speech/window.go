package speech

import "math"

const (
	// SampleRate is the rate the on-device models accept.
	SampleRate = 16000
	// DiarizeWindowSeconds is one pyannote segmentation window.
	DiarizeWindowSeconds = 10
	// EnglishChunkSeconds is one Moonshine pass.
	EnglishChunkSeconds = 5
	// HebrewChunkSeconds is one Zipformer streaming chunk (64 frames).
	HebrewChunkSeconds = 1.28
	// HebrewFlushSeconds is trailing silence so the streaming decoder emits
	// the last chunk. The model card uses 1.5 seconds.
	HebrewFlushSeconds = 1.5
	// MinEmbedSeconds is the shortest turn that receives an embedding.
	MinEmbedSeconds = 0.5
	// MinChunkSeconds is the shortest STT slice that is still decoded.
	MinChunkSeconds = 0.1
	// MaxEnrollSeconds is how much of an enrollment file is embedded.
	MaxEnrollSeconds = 30
	// MaxProcessSeconds is how much of a picked file is processed.
	// Longer files are cut here instead of being loaded as one pass.
	MaxProcessSeconds = 600
)

// Span is a half-open sample range [Start, End).
type Span struct {
	Start int
	End   int
}

// SampleCount rounds seconds to a sample count at SampleRate.
func SampleCount(seconds float64) int {
	if seconds <= 0 {
		return 0
	}
	n := int(math.Round(seconds * float64(SampleRate)))
	if n < 1 {
		return 1
	}
	return n
}

// PlanSpans splits [0, nSamples) into pieces of spanSamples.
// A trailing piece shorter than minTail is dropped when an earlier piece
// exists. A file shorter than minTail is one span so the caller can reject it.
func PlanSpans(nSamples, spanSamples, minTail int) []Span {
	if nSamples <= 0 || spanSamples <= 0 {
		return nil
	}
	if minTail < 1 {
		minTail = 1
	}
	out := make([]Span, 0, nSamples/spanSamples+1)
	for start := 0; start < nSamples; start += spanSamples {
		end := start + spanSamples
		if end > nSamples {
			end = nSamples
		}
		if end-start < minTail && len(out) > 0 {
			break
		}
		out = append(out, Span{Start: start, End: end})
		if end == nSamples {
			break
		}
	}
	return out
}

// DiarizeWindows plans 10 second segmentation windows over nSamples.
func DiarizeWindows(nSamples int) []Span {
	return PlanSpans(nSamples, SampleCount(DiarizeWindowSeconds), SampleCount(MinEmbedSeconds))
}

// EnglishChunks plans Moonshine slices over one turn.
func EnglishChunks(nSamples int) []Span {
	return PlanSpans(nSamples, SampleCount(EnglishChunkSeconds), SampleCount(MinChunkSeconds))
}

// HebrewChunks plans Zipformer slices over one turn.
func HebrewChunks(nSamples int) []Span {
	return PlanSpans(nSamples, SampleCount(HebrewChunkSeconds), SampleCount(MinChunkSeconds))
}
