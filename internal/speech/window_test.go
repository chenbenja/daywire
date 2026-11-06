package speech

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/speaker"
	"github.com/chenbenja/daywire/internal/testutil"
	"github.com/chenbenja/daywire/internal/voiceprint"
)

func TestPlanSpans(t *testing.T) {
	t.Parallel()
	rate := SampleRate
	cases := []struct {
		name    string
		samples int
		span    int
		minTail int
		want    []Span
	}{
		{
			name:    "two full windows and a kept tail",
			samples: 25 * rate, span: 10 * rate, minTail: rate / 2,
			want: []Span{{0, 10 * rate}, {10 * rate, 20 * rate}, {20 * rate, 25 * rate}},
		},
		{
			name:    "short tail dropped",
			samples: 10*rate + rate/5, span: 10 * rate, minTail: rate / 2,
			want: []Span{{0, 10 * rate}},
		},
		{
			name:    "shorter than the tail is still one span",
			samples: rate / 5, span: 10 * rate, minTail: rate / 2,
			want: []Span{{0, rate / 5}},
		},
		{
			name: "empty",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, PlanSpans(tc.samples, tc.span, tc.minTail))
		})
	}
}

func TestChunkSizes(t *testing.T) {
	t.Parallel()
	require.Equal(t, 10*SampleRate, SampleCount(DiarizeWindowSeconds))
	require.Equal(t, 5*SampleRate, SampleCount(EnglishChunkSeconds))
	require.Equal(t, 20480, SampleCount(HebrewChunkSeconds))
	require.Equal(t, 24000, SampleCount(HebrewFlushSeconds))
	require.InDelta(t, 1.28, HebrewChunkSeconds, 1e-9)
}

func TestLibriSpeechWindowsUseDocumentedScores(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	clip, err := speaker.LoadDemo(ctx, filepath.Join("..", "speaker", "testdata"))
	require.NoError(t, err)
	require.Equal(t, SampleRate, clip.SampleRate)

	// NeMo SpeakerNet cosines measured on these fixtures by the laptop spike:
	// the held-out utterance of reader 6930 is about 0.73, reader 1320 is about 0.24.
	const ownerScore = 0.73
	const otherScore = 0.24
	require.Equal(t, voiceprint.OwnerLabel, voiceprint.LabelTurn(ownerScore))
	require.Equal(t, voiceprint.OtherLabel, voiceprint.LabelTurn(otherScore))
	require.Equal(t, voiceprint.UnknownLabel, voiceprint.LabelTurn(0.37))
	require.Equal(t, voiceprint.UnknownLabel, voiceprint.LabelTurn(math.NaN()))

	windows := DiarizeWindows(len(clip.Mixed))
	require.NotEmpty(t, windows)
	require.Equal(t, 0, windows[0].Start)
	ownerEnd := int(clip.OwnerEnd * float64(clip.SampleRate))
	require.LessOrEqual(t, windows[0].End, ownerEnd)

	var sawOwner, sawOther bool
	for _, span := range windows {
		mid := float64(span.Start+span.End) / 2 / float64(clip.SampleRate)
		var label string
		switch {
		case mid < clip.OwnerEnd:
			label = voiceprint.LabelTurn(ownerScore)
			sawOwner = true
		case mid < clip.OtherStart:
			label = voiceprint.LabelTurn(math.NaN())
		default:
			label = voiceprint.LabelTurn(otherScore)
			sawOther = true
		}
		require.Contains(t, []string{voiceprint.OwnerLabel, voiceprint.OtherLabel, voiceprint.UnknownLabel}, label)
	}
	require.True(t, sawOwner)
	require.True(t, sawOther)

	// The enrollment clip itself is one embedding, not a diarized mix.
	enroll := DiarizeWindows(len(clip.Enroll))
	require.Len(t, enroll, 1)
	require.Less(t, float64(len(clip.Enroll))/float64(clip.SampleRate), float64(DiarizeWindowSeconds))
	require.GreaterOrEqual(t, float64(len(clip.Enroll))/float64(clip.SampleRate), MinEmbedSeconds)
}
