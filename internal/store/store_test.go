package store_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/crypt"
	"github.com/chenbenja/daywire/internal/event"
	"github.com/chenbenja/daywire/internal/store"
	"github.com/chenbenja/daywire/internal/testutil"
)

const marker = "REDACTED-SUMMARY-MARKER-9f3c2a"

func TestInsertQueryAndCiphertextAtRest(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	ctx := testutil.Context(t)
	st := testutil.OpenStore(t, cfg)
	now := time.Date(2026, 11, 5, 15, 0, 0, 0, time.UTC)

	events := []event.Event{
		sample(event.TypeCommitment, "c1", now.Add(-3*time.Hour), "commitment "+marker),
		sample(event.TypeAsk, "a1", now.Add(-2*time.Hour), "ask "+marker),
		sample(event.TypeVisit, "v1", now.Add(-time.Hour), "visit "+marker),
		sample(event.TypeNote, "n1", now.Add(-time.Minute), "note "+marker),
	}
	for _, ev := range events {
		require.NoError(t, st.InsertEvent(ctx, ev))
	}
	err := st.InsertEvent(ctx, events[0])
	require.Error(t, err)
	require.NotContains(t, err.Error(), marker)

	got, err := st.ListByOccurred(ctx, store.OccurredFilter{Type: event.TypeVisit, Limit: 10})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "v1", got[0].ID)
	require.Equal(t, "visit "+marker, got[0].Summary)
	require.True(t, got[0].OccurredAt.Equal(now.Add(-time.Hour)))

	ranged, err := st.ListByOccurred(ctx, store.OccurredFilter{
		From:  now.Add(-90 * time.Minute),
		To:    now.Add(-30 * time.Minute),
		Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, ranged, 1)
	require.Equal(t, "v1", ranged[0].ID)

	require.NoError(t, st.InsertNudge(ctx, store.Nudge{
		ID:        "nudge-1",
		CreatedAt: now,
		Channel:   "log",
		Delivered: true,
		Detail:    "logged",
		Title:     "Synthetic nudge",
		Message:   "nudge " + marker,
	}))
	require.NoError(t, st.Checkpoint(ctx))

	blob := readStoreFiles(t, st.Path())
	require.NotContains(t, string(blob), marker)
	require.NotContains(t, string(blob), "Synthetic nudge")
}

func TestWrongKeyCannotRead(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	ctx := testutil.Context(t)
	st := testutil.OpenStore(t, cfg)
	ev := sample(event.TypeNote, "note-1", time.Now().UTC(), "secret "+marker)
	require.NoError(t, st.InsertEvent(ctx, ev))
	require.NoError(t, st.Close())

	other, err := crypt.GenerateKey()
	require.NoError(t, err)
	st2, err := store.Open(ctx, cfg.DBPath(), other, testutil.Logger(t))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st2.Close()) })

	_, err = st2.ListByOccurred(ctx, store.OccurredFilter{Limit: 10})
	require.Error(t, err)
	require.NotContains(t, err.Error(), marker)
}

func TestCursorOrder(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	ctx := testutil.Context(t)
	st := testutil.OpenStore(t, cfg)
	base := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	for i, id := range []string{"e1", "e2", "e3"} {
		ev := sample(event.TypeNote, id, base.Add(time.Duration(i)*time.Minute), "summary "+id)
		ev.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		require.NoError(t, st.InsertEvent(ctx, ev))
	}
	first, err := st.ListSince(ctx, store.Cursor{}, 2)
	require.NoError(t, err)
	require.Equal(t, []string{"e1", "e2"}, ids(first))

	cursor := store.Cursor{CreatedAt: first[1].CreatedAt, ID: first[1].ID}
	require.Equal(t, cursor.String(), mustParse(t, cursor.String()).String())
	rest, err := st.ListSince(ctx, cursor, 10)
	require.NoError(t, err)
	require.Equal(t, []string{"e3"}, ids(rest))
	_, err = store.ParseCursor("not a cursor")
	require.Error(t, err)
}

func TestGetNotFound(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	st := testutil.OpenStore(t, cfg)
	_, err := st.GetEvent(testutil.Context(t), "missing-id")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func sample(typ event.Type, id string, at time.Time, summary string) event.Event {
	return event.Event{
		ID:         id,
		Type:       typ,
		SourceID:   "synthetic:test",
		OccurredAt: at,
		CreatedAt:  at,
		Summary:    summary,
	}
}

func ids(events []event.Event) []string {
	out := make([]string, 0, len(events))
	for _, ev := range events {
		out = append(out, ev.ID)
	}
	return out
}

func mustParse(t *testing.T, s string) store.Cursor {
	t.Helper()
	c, err := store.ParseCursor(s)
	require.NoError(t, err)
	return c
}

func readStoreFiles(t *testing.T, path string) []byte {
	t.Helper()
	var blob []byte
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		b, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		require.NoError(t, err)
		blob = append(blob, b...)
	}
	return blob
}
