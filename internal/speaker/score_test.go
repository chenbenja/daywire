package speaker

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCosineAndMean(t *testing.T) {
	t.Parallel()
	same, err := Cosine([]float32{1, 0}, []float32{2, 0})
	require.NoError(t, err)
	require.InDelta(t, 1, same, 1e-6)
	orth, err := Cosine([]float32{1, 0}, []float32{0, 1})
	require.NoError(t, err)
	require.InDelta(t, 0, orth, 1e-6)
	_, err = Cosine([]float32{1}, []float32{1, 0})
	require.Error(t, err)
	_, err = Cosine([]float32{float32(math.NaN())}, []float32{1})
	require.Error(t, err)

	mean, err := MeanEmbedding([][]float32{{1, 0}, {3, 2}})
	require.NoError(t, err)
	require.InDelta(t, 2, mean[0], 1e-5)
	require.InDelta(t, 1, mean[1], 1e-5)
}

func TestAssignOwner(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		scores  []ClusterScore
		want    int
		wantErr error
	}{
		{
			name:   "owner is the higher score",
			scores: []ClusterScore{{Speaker: 1, Score: 0.2}, {Speaker: 0, Score: 0.8}},
			want:   0,
		},
		{
			name:    "below the minimum",
			scores:  []ClusterScore{{Speaker: 0, Score: 0.2}, {Speaker: 1, Score: 0.4}},
			wantErr: ErrNoOwner,
		},
		{
			name:    "empty",
			wantErr: ErrNoOwner,
		},
		{
			name:    "tie",
			scores:  []ClusterScore{{Speaker: 0, Score: 0.8}, {Speaker: 1, Score: 0.8}},
			wantErr: ErrAmbiguousOwner,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := AssignOwner(tc.scores, DefaultOwnerMinScore)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got.Owner)
			require.Equal(t, OwnerLabel, got.Labels[tc.want])
			for spk, label := range got.Labels {
				if spk == tc.want {
					continue
				}
				require.Equal(t, OtherLabel, label)
			}
		})
	}
}
