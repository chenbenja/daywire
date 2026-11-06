package seed_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/event"
	"github.com/chenbenja/daywire/internal/seed"
	"github.com/chenbenja/daywire/internal/store"
	"github.com/chenbenja/daywire/internal/testutil"
)

func TestApplyIsIdempotent(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	ctx := testutil.Context(t)
	st := testutil.OpenStore(t, cfg)
	now := time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC)

	n, err := seed.Apply(ctx, st, now)
	require.NoError(t, err)
	require.Equal(t, 4, n)
	again, err := seed.Apply(ctx, st, now.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, 0, again)

	got, err := st.ListByOccurred(ctx, store.OccurredFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, got, 4)
	require.Equal(t, event.TypeNote, got[0].Type)
	for _, ev := range got {
		require.NotContains(t, ev.ID, "audio")
		require.NotEmpty(t, ev.Summary)
	}
}
