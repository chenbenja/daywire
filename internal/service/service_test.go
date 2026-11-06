package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/event"
	"github.com/chenbenja/daywire/internal/nudge"
	"github.com/chenbenja/daywire/internal/service"
	"github.com/chenbenja/daywire/internal/testutil"
)

func TestQueryValidation(t *testing.T) {
	svc := newService(t)
	ctx := testutil.Context(t)
	_, err := svc.Query(ctx, service.QueryInput{Type: "audio"})
	require.ErrorContains(t, err, "invalid event type")
	_, err = svc.Query(ctx, service.QueryInput{Since: "yesterday"})
	require.ErrorContains(t, err, "invalid since")
	_, err = svc.Query(ctx, service.QueryInput{Limit: 1000})
	require.ErrorContains(t, err, "invalid limit")
	_, err = svc.Query(ctx, service.QueryInput{
		Since: "2026-11-05T12:00:00Z",
		Until: "2026-11-05T11:00:00Z",
	})
	require.ErrorContains(t, err, "until is before since")
}

func TestSubscribeWaitsForInsert(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	ctx := testutil.Context(t)
	st := testutil.OpenStore(t, cfg)
	svc := service.New(st, nudge.New("", "", testutil.Logger(t)), testutil.Logger(t))

	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(250 * time.Millisecond)
		ev := event.Event{
			ID:         "late-note",
			Type:       event.TypeNote,
			SourceID:   "synthetic:note",
			OccurredAt: time.Now().UTC(),
			CreatedAt:  time.Now().UTC(),
			Summary:    "Note: a synthetic follow-up.",
		}
		require.NoError(t, st.InsertEvent(ctx, ev))
	}()

	out, err := svc.Subscribe(ctx, service.SubscribeInput{WaitMS: 3000, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, "poll", out.Mode)
	require.Len(t, out.Events, 1)
	require.Equal(t, "late-note", out.Events[0].ID)
	require.NotEmpty(t, out.Cursor)
	<-done

	next, err := svc.Subscribe(ctx, service.SubscribeInput{Cursor: out.Cursor, Limit: 10})
	require.NoError(t, err)
	require.Empty(t, next.Events)
	require.Equal(t, out.Cursor, next.Cursor)
}

func TestProposeLogFallback(t *testing.T) {
	svc := newService(t)
	out, err := svc.Propose(testutil.Context(t), service.ProposeInput{
		Title:   "Synthetic nudge",
		Message: "Pack the spare charger.",
	})
	require.NoError(t, err)
	require.True(t, out.Delivered)
	require.Equal(t, "log", out.Channel)
	require.Equal(t, "logged", out.Detail)
	require.True(t, event.ValidID(out.ID))
}

func newService(t *testing.T) *service.Service {
	t.Helper()
	cfg := testutil.LoadConfig(t)
	st := testutil.OpenStore(t, cfg)
	return service.New(st, nudge.New("", "", testutil.Logger(t)), testutil.Logger(t))
}
