//go:build cgo

package speaker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/testutil"
)

func TestEnrollAndLabelOwner(t *testing.T) {
	// A cold model download plus ONNX load does not fit in testutil's 30s budget.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	cache := modelCache(t)
	engine, err := NewEngine(ctx, cache, testutil.Logger(t))
	if err != nil {
		require.ErrorIs(t, err, ErrModelsUnavailable)
		t.Skip(err.Error())
	}
	t.Cleanup(func() { require.NoError(t, engine.Close()) })

	clip, err := LoadDemo(ctx, "testdata")
	require.NoError(t, err)
	dir := t.TempDir()
	key := testKey(t)
	dim, err := Enroll(ctx, engine, dir, key, "owner", clip.Enroll, clip.SampleRate)
	require.NoError(t, err)
	require.Equal(t, 256, dim)
	t.Logf("enroll succeeded dim=%d", dim)

	res, err := LabelClip(ctx, engine, dir, key, "owner", clip.Mixed, clip.SampleRate, 0)
	require.NoError(t, err)
	require.NotEmpty(t, res.Turns)

	var ownerN, otherN int
	var bestOwner, bestOther float64
	bestOther = -1
	for _, turn := range res.Turns {
		mid := (turn.Start + turn.End) / 2
		switch turn.Label {
		case OwnerLabel:
			ownerN++
			require.GreaterOrEqual(t, turn.Score, DefaultOwnerMinScore)
			require.Less(t, mid, clip.OwnerEnd+0.35)
			if turn.Score > bestOwner {
				bestOwner = turn.Score
			}
		case OtherLabel:
			otherN++
			require.Less(t, turn.Score, DefaultOwnerMinScore)
			require.Greater(t, mid, clip.OtherStart-0.35)
			if turn.Score > bestOther {
				bestOther = turn.Score
			}
		default:
			t.Fatalf("unexpected label %q", turn.Label)
		}
		t.Logf("speaker=%d start=%.3f end=%.3f label=%s score=%.3f", turn.Speaker, turn.Start, turn.End, turn.Label, turn.Score)
	}
	require.Greater(t, ownerN, 0)
	require.Greater(t, otherN, 0)
	require.Greater(t, bestOwner, bestOther+0.3)
	require.Equal(t, OwnerLabel, labelOf(res, res.Owner))
	t.Logf("owner cluster labeled speaker=%d", res.Owner)

	_, err = os.Stat(filepath.Join(dir, "events.db"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func labelOf(res Result, speakerID int) string {
	for _, turn := range res.Turns {
		if turn.Speaker == speakerID {
			return turn.Label
		}
	}
	return ""
}

func modelCache(t *testing.T) string {
	t.Helper()
	root, err := moduleRoot()
	require.NoError(t, err)
	dir := filepath.Join(root, "output", "speaker-models")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	return dir
}
