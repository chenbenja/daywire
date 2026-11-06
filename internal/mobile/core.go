package mobile

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/chenbenja/daywire/internal/config"
	"github.com/chenbenja/daywire/internal/event"
	"github.com/chenbenja/daywire/internal/proposal"
	"github.com/chenbenja/daywire/internal/store"
)

const (
	defaultLimit  = 50
	callTimeout   = 15 * time.Second
	visitSourceID = "android:location"
	localShown    = "shown"
	localNotShown = "not shown"
)

// Core is an open on-device store. It does not listen on a port and it does
// not import the MCP server.
type Core struct {
	store   *store.Store
	log     *slog.Logger
	dataDir string
	key     []byte
}

// Open opens or creates the store under dataDir. Summaries and nudge text are
// AES-256-GCM; identifiers, types, and times stay plaintext. The directory is
// created at mode 0700 and the key file follows the Week 1 rules. An empty
// directory is refused so a missing Android path cannot fall through to the
// laptop default. The key file is not wrapped in the Android Keystore.
func Open(dataDir string) (*Core, error) {
	return OpenWithLogger(dataDir, newLogger())
}

// OpenWithLogger is Open with a caller-supplied logger. The Android bind does
// not export this; tests use it to prove coordinates and nudge text stay out
// of the log.
func OpenWithLogger(dataDir string, logger *slog.Logger) (*Core, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, fmt.Errorf("data dir is empty")
	}
	if logger == nil {
		logger = newLogger()
	}
	cfg, err := config.Load(config.Options{DataDir: dataDir})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	st, err := store.Open(ctx, cfg.DBPath(), cfg.DataKey, logger)
	if err != nil {
		return nil, err
	}
	logger.Info("daywire core open", "key_source", cfg.KeySource)
	return &Core{store: st, log: logger, dataDir: cfg.DataDir, key: cfg.DataKey}, nil
}

// Close closes the database.
func (c *Core) Close() error {
	if c == nil || c.store == nil {
		return nil
	}
	err := c.store.Close()
	c.store = nil
	for i := range c.key {
		c.key[i] = 0
	}
	c.key = nil
	if err != nil {
		return fmt.Errorf("close core: %w", err)
	}
	return nil
}

// DBPath returns the SQLite file path.
func (c *Core) DBPath() string {
	if c == nil || c.store == nil {
		return ""
	}
	return c.store.Path()
}

// Checkpoint flushes the write-ahead log into the database file.
func (c *Core) Checkpoint() error {
	if err := c.ready(); err != nil {
		return err
	}
	return c.with(func(ctx context.Context) error {
		return c.store.Checkpoint(ctx)
	})
}

// InsertRedactedNote stores a note whose summary the caller has already
// redacted. The Android shell may call it only with the fixed synthetic
// sentence. Notification capture must use InsertNotification, which does not
// take a title or a body.
func (c *Core) InsertRedactedNote(sourceID, summary string, occurredUnixMilli int64) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	return c.insert(event.TypeNote, strings.TrimSpace(sourceID), strings.TrimSpace(summary), occurredUnixMilli)
}

// InsertNotification stores a note for an allowed notification. packageName
// and appLabel identify the app. category is the platform category constant,
// or empty. This method has no title or body parameter.
func (c *Core) InsertNotification(packageNameValue, appLabel, category string, postedUnixMilli int64) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	summary, err := NotificationSummary(appLabel, packageNameValue, category)
	if err != nil {
		return "", err
	}
	source, err := notificationSource(packageNameValue)
	if err != nil {
		return "", err
	}
	return c.insert(event.TypeNote, source, summary, postedUnixMilli)
}

// InsertCoarseVisit stores a visit. latitude and longitude are quantized
// inside CoarseVisitSummary. They are not written to the log.
func (c *Core) InsertCoarseVisit(latitude, longitude float64, occurredUnixMilli int64) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	summary, err := CoarseVisitSummary(latitude, longitude)
	if err != nil {
		return "", err
	}
	return c.insert(event.TypeVisit, visitSourceID, summary, occurredUnixMilli)
}

// ListEventsJSON returns events as a JSON array, newest occurred_at first.
// eventType empty matches every type, including speech turns for the phone
// UI. limit 0 uses 50. The maximum is the store maximum. This is not the
// assistant query path.
func (c *Core) ListEventsJSON(eventType string, limit int) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 0 || limit > store.MaxLimit {
		return "", fmt.Errorf("invalid limit")
	}
	var typ event.Type
	if strings.TrimSpace(eventType) != "" {
		typ = event.Type(strings.TrimSpace(eventType))
		if !typ.Valid() {
			return "", fmt.Errorf("invalid event type")
		}
	}
	var rows []event.Event
	err := c.with(func(ctx context.Context) error {
		var qerr error
		rows, qerr = c.store.ListByOccurred(ctx, store.OccurredFilter{Type: typ, Limit: limit})
		return qerr
	})
	if err != nil {
		return "", err
	}
	views := make([]eventView, 0, len(rows))
	for _, ev := range rows {
		views = append(views, eventView{
			ID:         ev.ID,
			Type:       string(ev.Type),
			SourceID:   ev.SourceID,
			OccurredAt: ev.OccurredAt.UTC().Format(time.RFC3339Nano),
			CreatedAt:  ev.CreatedAt.UTC().Format(time.RFC3339Nano),
			Summary:    ev.Summary,
		})
	}
	body, err := json.Marshal(views)
	if err != nil {
		return "", fmt.Errorf("encode events: %w", err)
	}
	return string(body), nil
}

// RecordLocalNudge stores a nudge the phone displayed itself. It validates
// the text with the Week 1 rules and does not call the ntfy sender. delivered
// false records detail "not shown".
func (c *Core) RecordLocalNudge(title, message string, delivered bool) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	text := proposal.Proposal{Title: strings.TrimSpace(title), Message: strings.TrimSpace(message)}
	if err := text.Validate(); err != nil {
		return "", err
	}
	id, err := event.NewID()
	if err != nil {
		return "", err
	}
	detail := localNotShown
	if delivered {
		detail = localShown
	}
	err = c.with(func(ctx context.Context) error {
		return c.store.InsertNudge(ctx, store.Nudge{
			ID:        id,
			CreatedAt: time.Now().UTC(),
			Channel:   store.ChannelLocal,
			Delivered: delivered,
			Detail:    detail,
			Title:     text.Title,
			Message:   text.Message,
		})
	})
	if err != nil {
		return "", err
	}
	c.log.Info("local nudge recorded", "nudge_id", id, "delivered", delivered)
	return id, nil
}

// NudgeJSON returns one stored nudge, including its title and message, as JSON.
func (c *Core) NudgeJSON(id string) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	var n store.Nudge
	err := c.with(func(ctx context.Context) error {
		var qerr error
		n, qerr = c.store.GetNudge(ctx, strings.TrimSpace(id))
		return qerr
	})
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(nudgeView{
		ID:        n.ID,
		Channel:   n.Channel,
		Delivered: n.Delivered,
		Detail:    n.Detail,
		Title:     n.Title,
		Message:   n.Message,
		CreatedAt: n.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return "", fmt.Errorf("encode nudge: %w", err)
	}
	return string(body), nil
}

func (c *Core) insert(typ event.Type, sourceID, summary string, occurredUnixMilli int64) (string, error) {
	id, err := event.NewID()
	if err != nil {
		return "", err
	}
	ev := event.Event{
		ID:         id,
		Type:       typ,
		SourceID:   sourceID,
		OccurredAt: unixMilliOrNow(occurredUnixMilli),
		CreatedAt:  time.Now().UTC(),
		Summary:    summary,
	}
	err = c.with(func(ctx context.Context) error {
		return c.store.InsertEvent(ctx, ev)
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func (c *Core) ready() error {
	if c == nil || c.store == nil {
		return fmt.Errorf("core is closed")
	}
	return nil
}

func (c *Core) with(fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	return fn(ctx)
}

func unixMilliOrNow(unixMilli int64) time.Time {
	if unixMilli <= 0 {
		return time.Now().UTC()
	}
	return time.UnixMilli(unixMilli).UTC()
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

type eventView struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	SourceID   string `json:"source_id"`
	OccurredAt string `json:"occurred_at"`
	CreatedAt  string `json:"created_at"`
	Summary    string `json:"summary"`
}

type nudgeView struct {
	ID        string `json:"id"`
	Channel   string `json:"channel"`
	Delivered bool   `json:"delivered"`
	Detail    string `json:"detail"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}
