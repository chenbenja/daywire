package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chenbenja/daywire/internal/event"
	"github.com/chenbenja/daywire/internal/store"
)

// Events returns the Week 1 demo events. They are synthetic calendar, mail,
// visit, and note facts. They contain no real personal data and no audio.
func Events(now time.Time) []event.Event {
	now = now.UTC().Truncate(time.Microsecond)
	return []event.Event{
		{
			ID:         "seed-commitment-1",
			Type:       event.TypeCommitment,
			SourceID:   "synthetic:calendar",
			OccurredAt: now.Add(-2 * time.Hour),
			CreatedAt:  now.Add(-2 * time.Hour),
			Summary:    "Calendar: send the weekly status note on Friday.",
		},
		{
			ID:         "seed-ask-1",
			Type:       event.TypeAsk,
			SourceID:   "synthetic:mail",
			OccurredAt: now.Add(-90 * time.Minute),
			CreatedAt:  now.Add(-90 * time.Minute),
			Summary:    "Mail: please confirm a meeting place for Saturday.",
		},
		{
			ID:         "seed-visit-1",
			Type:       event.TypeVisit,
			SourceID:   "synthetic:visit",
			OccurredAt: now.Add(-30 * time.Minute),
			CreatedAt:  now.Add(-30 * time.Minute),
			Summary:    "Visit: arrived at the neighborhood library.",
		},
		{
			ID:         "seed-note-1",
			Type:       event.TypeNote,
			SourceID:   "synthetic:note",
			OccurredAt: now.Add(-5 * time.Minute),
			CreatedAt:  now.Add(-5 * time.Minute),
			Summary:    "Note: pack the spare charger before leaving.",
		},
	}
}

// Apply inserts any demo events that are not already stored. It is safe to
// run more than once.
func Apply(ctx context.Context, st *store.Store, now time.Time) (int, error) {
	inserted := 0
	for _, ev := range Events(now) {
		_, err := st.GetEvent(ctx, ev.ID)
		if err == nil {
			continue
		}
		if !errors.Is(err, store.ErrNotFound) {
			return inserted, fmt.Errorf("check seed event %s: %w", ev.ID, err)
		}
		if err := st.InsertEvent(ctx, ev); err != nil {
			return inserted, fmt.Errorf("insert seed event %s: %w", ev.ID, err)
		}
		inserted++
	}
	return inserted, nil
}
