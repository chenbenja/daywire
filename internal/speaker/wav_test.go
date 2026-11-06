package speaker

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/testutil"
)

func TestWAVRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	in := []float32{0, 0.5, -0.5, 0.25}
	path := t.TempDir() + "/clip.wav"
	require.NoError(t, WriteWAV(ctx, path, 8000, in))
	rate, out, err := ReadWAV(ctx, path)
	require.NoError(t, err)
	require.Equal(t, 8000, rate)
	require.Len(t, out, len(in))
	for i := range in {
		require.InDelta(t, in[i], out[i], 1.0/32767)
	}
}

func TestReadWAVRejectsNonPCM(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	path := t.TempDir() + "/nope.wav"
	require.NoError(t, os.WriteFile(path, make([]byte, 64), 0o644))
	_, _, err := ReadWAV(ctx, path)
	require.Error(t, err)
}
