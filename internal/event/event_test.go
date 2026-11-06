package event_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/event"
)

func TestTypeValid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   event.Type
		want bool
	}{
		{event.TypeCommitment, true},
		{event.TypeAsk, true},
		{event.TypeVisit, true},
		{event.TypeNote, true},
		{"audio", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(string(tc.in), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.in.Valid())
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	base := sampleEvent()
	cases := []struct {
		name    string
		mutate  func(*event.Event)
		wantErr string
	}{
		{name: "ok"},
		{name: "bad id", mutate: func(e *event.Event) { e.ID = "has space" }, wantErr: "invalid id"},
		{name: "bad type", mutate: func(e *event.Event) { e.Type = "audio" }, wantErr: "invalid event type"},
		{name: "bad source", mutate: func(e *event.Event) { e.SourceID = "mail body" }, wantErr: "invalid source id"},
		{name: "no occurred", mutate: func(e *event.Event) { e.OccurredAt = time.Time{} }, wantErr: "occurred_at is empty"},
		{name: "no created", mutate: func(e *event.Event) { e.CreatedAt = time.Time{} }, wantErr: "created_at is empty"},
		{name: "empty summary", mutate: func(e *event.Event) { e.Summary = "  " }, wantErr: "summary is empty"},
		{name: "long summary", mutate: func(e *event.Event) { e.Summary = strings.Repeat("a", event.MaxSummaryBytes+1) }, wantErr: "summary is too long"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ev := base
			if tc.mutate != nil {
				tc.mutate(&ev)
			}
			err := ev.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
			require.NotContains(t, err.Error(), "weekly status")
		})
	}
}

func TestNewID(t *testing.T) {
	a, err := event.NewID()
	require.NoError(t, err)
	b, err := event.NewID()
	require.NoError(t, err)
	require.NotEqual(t, a, b)
	require.NoError(t, (event.Event{
		ID:         a,
		Type:       event.TypeNote,
		SourceID:   "synthetic:note",
		OccurredAt: time.Now().UTC(),
		CreatedAt:  time.Now().UTC(),
		Summary:    "ok",
	}).Validate())
}

func sampleEvent() event.Event {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	return event.Event{
		ID:         "seed-note-1",
		Type:       event.TypeNote,
		SourceID:   "synthetic:note",
		OccurredAt: now,
		CreatedAt:  now,
		Summary:    "Calendar: send the weekly status note on Friday.",
	}
}
