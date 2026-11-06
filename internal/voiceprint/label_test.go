package voiceprint

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/testutil"
)

func TestLabelTurn(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		score float64
		want  string
	}{
		{name: "fixture owner", score: 0.73, want: OwnerLabel},
		{name: "at the owner cut", score: OwnerMinScore, want: OwnerLabel},
		{name: "fixture other", score: 0.24, want: OtherLabel},
		{name: "at the other cut", score: OtherMaxScore, want: OtherLabel},
		{name: "between the cuts", score: 0.37, want: UnknownLabel},
		{name: "just under owner", score: OwnerMinScore - 0.001, want: UnknownLabel},
		{name: "just over other", score: OtherMaxScore + 0.001, want: UnknownLabel},
		{name: "nan", score: math.NaN(), want: UnknownLabel},
		{name: "inf", score: math.Inf(1), want: UnknownLabel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, LabelTurn(tc.score))
		})
	}
}

func TestMatchOwnerOtherUnknown(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	dir := t.TempDir()
	key := testKey(t)
	require.NoError(t, Save(ctx, dir, key, OwnerID, []float32{1, 0}))

	owner, err := Match(ctx, dir, key, OwnerID, []float32{0.8, 0.1})
	require.NoError(t, err)
	require.Equal(t, OwnerLabel, owner)

	other, err := Match(ctx, dir, key, OwnerID, []float32{0, 1})
	require.NoError(t, err)
	require.Equal(t, OtherLabel, other)

	// Cosine with (1,1) against (1,0) is about 0.707, which is owner.
	// A vector at cosine ~0.37 sits in the unknown band: (0.37, 0.929).
	unknown, err := Match(ctx, dir, key, OwnerID, []float32{0.37, 0.929})
	require.NoError(t, err)
	require.Equal(t, UnknownLabel, unknown)

	_, err = Match(ctx, dir, key, "missing", []float32{1, 0})
	require.ErrorIs(t, err, ErrNotEnrolled)
}
