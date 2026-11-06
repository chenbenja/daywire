package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/chenbenja/daywire/internal/event"
	"github.com/chenbenja/daywire/internal/nudge"
	"github.com/chenbenja/daywire/internal/store"
)

const (
	// DefaultLimit is the page size used when the caller omits a limit.
	DefaultLimit = 50
	// MaxWait is the longest subscribe long-poll.
	MaxWait = 25 * time.Second
)

// EventView is a redacted event as returned to assistants and local HTTP clients.
type EventView struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	SourceID   string `json:"source_id"`
	OccurredAt string `json:"occurred_at"`
	CreatedAt  string `json:"created_at"`
	Summary    string `json:"summary"`
}

// QueryInput filters events by occurred_at.
type QueryInput struct {
	Type  string `json:"type,omitempty" jsonschema:"commitment, ask, visit, or note; omit for every assistant-visible type. Speech turns are not included."`
	Since string `json:"since,omitempty" jsonschema:"RFC3339 inclusive lower bound on occurred_at"`
	Until string `json:"until,omitempty" jsonschema:"RFC3339 inclusive upper bound on occurred_at"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum events; default 50, max 200"`
}

// QueryOutput is the query_events result.
type QueryOutput struct {
	Events []EventView `json:"events"`
}

// SubscribeInput polls for events created after Cursor.
type SubscribeInput struct {
	Cursor string `json:"cursor,omitempty" jsonschema:"opaque cursor from a previous subscribe call; empty starts at the oldest event"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum events; default 50, max 200"`
	WaitMS int    `json:"wait_ms,omitempty" jsonschema:"long-poll up to this many milliseconds when no events are ready; max 25000"`
}

// SubscribeOutput is a poll page. Mode is always "poll" in Week 1.
type SubscribeOutput struct {
	Events []EventView `json:"events"`
	Cursor string      `json:"cursor"`
	Mode   string      `json:"mode"`
}

// ProposeInput is a redacted nudge proposal.
type ProposeInput struct {
	Title    string `json:"title" jsonschema:"short redacted title"`
	Message  string `json:"message" jsonschema:"redacted nudge text; never call audio"`
	Priority int    `json:"priority,omitempty" jsonschema:"optional ntfy priority from 1 to 5"`
}

// ProposeOutput is the delivery status.
type ProposeOutput struct {
	ID        string `json:"id"`
	Channel   string `json:"channel"`
	Delivered bool   `json:"delivered"`
	Detail    string `json:"detail"`
}

// Service is the Week 1 use-case API.
type Service struct {
	store  *store.Store
	nudges *nudge.Sender
	log    *slog.Logger
}

// New returns a service. st and nudges must be non-nil.
func New(st *store.Store, nudges *nudge.Sender, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Service{store: st, nudges: nudges, log: logger}
}

// Query returns redacted events matching in.
func (s *Service) Query(ctx context.Context, in QueryInput) (QueryOutput, error) {
	limit, err := normalizeLimit(in.Limit)
	if err != nil {
		return QueryOutput{}, err
	}
	var typ event.Type
	if strings.TrimSpace(in.Type) != "" {
		typ = event.Type(strings.TrimSpace(in.Type))
		if !typ.AssistantVisible() {
			return QueryOutput{}, fmt.Errorf("invalid event type")
		}
	}
	from, err := parseTime(in.Since)
	if err != nil {
		return QueryOutput{}, fmt.Errorf("invalid since")
	}
	to, err := parseTime(in.Until)
	if err != nil {
		return QueryOutput{}, fmt.Errorf("invalid until")
	}
	if !from.IsZero() && !to.IsZero() && to.Before(from) {
		return QueryOutput{}, fmt.Errorf("until is before since")
	}
	rows, err := s.store.ListByOccurred(ctx, store.OccurredFilter{
		Type:      typ,
		From:      from,
		To:        to,
		Limit:     limit,
		Assistant: true,
	})
	if err != nil {
		return QueryOutput{}, err
	}
	return QueryOutput{Events: views(rows)}, nil
}

// Subscribe returns events created after in.Cursor. It long-polls when WaitMS
// is positive and no events are ready. This is a poll, not a push subscription.
func (s *Service) Subscribe(ctx context.Context, in SubscribeInput) (SubscribeOutput, error) {
	limit, err := normalizeLimit(in.Limit)
	if err != nil {
		return SubscribeOutput{}, err
	}
	if in.WaitMS < 0 {
		return SubscribeOutput{}, fmt.Errorf("invalid wait_ms")
	}
	wait := time.Duration(in.WaitMS) * time.Millisecond
	if wait > MaxWait {
		return SubscribeOutput{}, fmt.Errorf("wait_ms exceeds 25000")
	}
	cursor, err := store.ParseCursor(strings.TrimSpace(in.Cursor))
	if err != nil {
		return SubscribeOutput{}, err
	}
	deadline := time.Now().Add(wait)
	for {
		out, err := s.poll(ctx, cursor, limit)
		if err != nil {
			return SubscribeOutput{}, err
		}
		if len(out.Events) > 0 || wait <= 0 || !time.Now().Before(deadline) {
			if out.Cursor == "" {
				out.Cursor = cursor.String()
			}
			return out, nil
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return SubscribeOutput{}, fmt.Errorf("subscribe: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

// Propose delivers a nudge and records the attempt.
func (s *Service) Propose(ctx context.Context, in ProposeInput) (ProposeOutput, error) {
	proposal := nudge.Proposal{Title: strings.TrimSpace(in.Title), Message: strings.TrimSpace(in.Message), Priority: in.Priority}
	res, err := s.nudges.Deliver(ctx, proposal)
	if res.ID != "" {
		recErr := s.store.InsertNudge(ctx, store.Nudge{
			ID:        res.ID,
			CreatedAt: time.Now().UTC(),
			Channel:   res.Channel,
			Delivered: res.Delivered,
			Detail:    res.Detail,
			Title:     proposal.Title,
			Message:   proposal.Message,
		})
		if recErr != nil {
			s.log.ErrorContext(ctx, "record nudge", "nudge_id", res.ID, "channel", res.Channel)
			if res.Detail == "" {
				res.Detail = "audit record failed"
			} else {
				res.Detail += "; audit record failed"
			}
		}
	}
	if err != nil && res.ID == "" {
		return ProposeOutput{}, err
	}
	if err != nil {
		s.log.ErrorContext(ctx, "nudge not delivered", "nudge_id", res.ID, "channel", res.Channel, "detail", res.Detail)
	}
	return ProposeOutput{
		ID:        res.ID,
		Channel:   res.Channel,
		Delivered: res.Delivered,
		Detail:    res.Detail,
	}, nil
}

func (s *Service) poll(ctx context.Context, cursor store.Cursor, limit int) (SubscribeOutput, error) {
	rows, err := s.store.ListAssistantSince(ctx, cursor, limit)
	if err != nil {
		return SubscribeOutput{}, err
	}
	out := SubscribeOutput{Events: views(rows), Cursor: cursor.String(), Mode: "poll"}
	if len(rows) > 0 {
		last := rows[len(rows)-1]
		out.Cursor = (store.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}).String()
	}
	return out, nil
}

func views(rows []event.Event) []EventView {
	out := make([]EventView, 0, len(rows))
	for _, ev := range rows {
		out = append(out, EventView{
			ID:         ev.ID,
			Type:       string(ev.Type),
			SourceID:   ev.SourceID,
			OccurredAt: ev.OccurredAt.UTC().Format(time.RFC3339Nano),
			CreatedAt:  ev.CreatedAt.UTC().Format(time.RFC3339Nano),
			Summary:    ev.Summary,
		})
	}
	return out
}

func normalizeLimit(limit int) (int, error) {
	if limit == 0 {
		return DefaultLimit, nil
	}
	if limit < 0 || limit > store.MaxLimit {
		return 0, fmt.Errorf("invalid limit")
	}
	return limit, nil
}

func parseTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}
