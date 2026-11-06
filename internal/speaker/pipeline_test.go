package speaker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/testutil"
)

type fakeEngine struct {
	owner []float32
	other []float32
}

func (f *fakeEngine) Embed(_ context.Context, samples []float32, sampleRate int) ([]float32, error) {
	if sampleRate != SampleRate {
		return nil, ErrEngineUnavailable
	}
	var sum float64
	for _, s := range samples {
		sum += float64(s)
	}
	if sum >= 0 {
		return append([]float32(nil), f.owner...), nil
	}
	return append([]float32(nil), f.other...), nil
}

func (f *fakeEngine) Diarize(_ context.Context, _ []float32, _ int, _ int) ([]Segment, error) {
	return []Segment{
		{Speaker: 0, Start: 0, End: 2},
		{Speaker: 1, Start: 2.5, End: 4.5},
	}, nil
}

func (f *fakeEngine) Close() error { return nil }

func TestLabelClipWithFakeEngine(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	engine := &fakeEngine{owner: []float32{1, 0}, other: []float32{0, 1}}
	dir := t.TempDir()
	key := testKey(t)
	enroll := []float32{0.2, 0.2, 0.2, 0.2}
	// Pad to at least 0.5s so a real engine would accept it. The fake does not care.
	dim, err := Enroll(ctx, engine, dir, key, "owner", enroll, SampleRate)
	require.NoError(t, err)
	require.Equal(t, 2, dim)

	samples := make([]float32, SampleRate*5)
	for i := 0; i < SampleRate*2; i++ {
		samples[i] = 0.1
	}
	for i := int(2.5 * float64(SampleRate)); i < int(4.5*float64(SampleRate)); i++ {
		samples[i] = -0.1
	}
	res, err := LabelClip(ctx, engine, dir, key, "owner", samples, SampleRate, 0)
	require.NoError(t, err)
	require.Equal(t, 0, res.Owner)
	require.Len(t, res.Turns, 2)
	require.Equal(t, OwnerLabel, res.Turns[0].Label)
	require.Equal(t, OtherLabel, res.Turns[1].Label)
	require.InDelta(t, 1, res.Turns[0].Score, 1e-6)
	require.InDelta(t, 0, res.Turns[1].Score, 1e-6)
}

func TestLabelClipMissingProfile(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	engine := &fakeEngine{owner: []float32{1, 0}, other: []float32{0, 1}}
	_, err := LabelClip(ctx, engine, t.TempDir(), testKey(t), "owner", make([]float32, SampleRate), SampleRate, 0)
	require.ErrorIs(t, err, ErrNotEnrolled)
}
