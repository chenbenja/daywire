package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/chenbenja/daywire/internal/crypt"
	"github.com/chenbenja/daywire/internal/event"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS events (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  source_id TEXT NOT NULL,
  occurred_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  summary_nonce BLOB NOT NULL,
  summary_ct BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_occurred ON events (occurred_at, type);
CREATE INDEX IF NOT EXISTS idx_events_created ON events (created_at, id);
CREATE TABLE IF NOT EXISTS nudges (
  id TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL,
  channel TEXT NOT NULL,
  delivered INTEGER NOT NULL,
  detail TEXT NOT NULL,
  title_nonce BLOB NOT NULL,
  title_ct BLOB NOT NULL,
  message_nonce BLOB NOT NULL,
  message_ct BLOB NOT NULL
);
`

// MaxLimit is the largest page the store will return.
const MaxLimit = 200

// ErrNotFound is returned when an event id is absent.
var ErrNotFound = errors.New("event not found")

// Store is an append-only event database.
type Store struct {
	db   *sql.DB
	box  *crypt.Box
	path string
	log  *slog.Logger
}

// OccurredFilter selects events by type and occurred_at.
// Zero times are open bounds. Type empty matches every type.
type OccurredFilter struct {
	Type  event.Type
	From  time.Time
	To    time.Time
	Limit int
}

// Cursor is an exclusive lower bound on (created_at, id), in ascending order.
type Cursor struct {
	CreatedAt time.Time
	ID        string
}

// String encodes c for clients. The zero cursor is the empty string.
func (c Cursor) String() string {
	if c.ID == "" {
		return ""
	}
	return strconv.FormatInt(c.CreatedAt.UTC().UnixMicro(), 10) + "|" + c.ID
}

// ParseCursor decodes a cursor from String. An empty string is the start.
func ParseCursor(s string) (Cursor, error) {
	if s == "" {
		return Cursor{}, nil
	}
	if len(s) > 128 {
		return Cursor{}, fmt.Errorf("invalid cursor")
	}
	microsStr, id, ok := strings.Cut(s, "|")
	if !ok || id == "" || strings.Contains(id, "|") {
		return Cursor{}, fmt.Errorf("invalid cursor")
	}
	micros, err := strconv.ParseInt(microsStr, 10, 64)
	if err != nil || micros <= 0 {
		return Cursor{}, fmt.Errorf("invalid cursor")
	}
	return Cursor{CreatedAt: time.UnixMicro(micros).UTC(), ID: id}, nil
}

// Nudge is a delivery record. Title and Message are encrypted at rest.
type Nudge struct {
	ID        string
	CreatedAt time.Time
	Channel   string
	Delivered bool
	Detail    string
	Title     string
	Message   string
}

// Open opens or creates the database at path using key.
func Open(ctx context.Context, path string, key []byte, logger *slog.Logger) (*Store, error) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	box, err := crypt.NewBox(key)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create store directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("restrict store directory: %w", err)
	}
	db, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("restrict store file: %w", err)
	}
	logger.DebugContext(ctx, "store ready")
	return &Store{db: db, box: box, path: path, log: logger}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close store: %w", err)
	}
	return nil
}

// Ping checks that the database accepts queries.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping store: %w", err)
	}
	return nil
}

// Path returns the database file path.
func (s *Store) Path() string {
	return s.path
}

// Checkpoint flushes the write-ahead log into the database file.
func (s *Store) Checkpoint(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("checkpoint store: %w", err)
	}
	return nil
}

// InsertEvent appends ev. It rejects invalid events and duplicate ids.
func (s *Store) InsertEvent(ctx context.Context, ev event.Event) error {
	ev = ev.Normalize()
	if err := ev.Validate(); err != nil {
		return err
	}
	nonce, ct, err := s.box.Seal(crypt.AAD(ev.ID, "summary"), []byte(ev.Summary))
	if err != nil {
		return fmt.Errorf("seal event %s: %w", ev.ID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO events (
			id, type, source_id, occurred_at, created_at, summary_nonce, summary_ct
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ev.ID,
		string(ev.Type),
		ev.SourceID,
		ev.OccurredAt.UnixMicro(),
		ev.CreatedAt.UnixMicro(),
		nonce,
		ct,
	)
	if err != nil {
		return fmt.Errorf("insert event %s: %w", ev.ID, err)
	}
	s.log.DebugContext(ctx, "event inserted", "event_id", ev.ID, "type", ev.Type)
	return nil
}

// GetEvent returns the event with id.
func (s *Store) GetEvent(ctx context.Context, id string) (event.Event, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, type, source_id, occurred_at, created_at, summary_nonce, summary_ct
		FROM events WHERE id = ?`, id)
	ev, err := s.scanEvent(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return event.Event{}, ErrNotFound
		}
		return event.Event{}, err
	}
	return ev, nil
}

// ListByOccurred returns events matching f, newest occurred_at first.
func (s *Store) ListByOccurred(ctx context.Context, f OccurredFilter) ([]event.Event, error) {
	if err := checkLimit(f.Limit); err != nil {
		return nil, err
	}
	q := `
		SELECT id, type, source_id, occurred_at, created_at, summary_nonce, summary_ct
		FROM events WHERE 1=1`
	args := make([]any, 0, 4)
	if f.Type != "" {
		q += ` AND type = ?`
		args = append(args, string(f.Type))
	}
	if !f.From.IsZero() {
		q += ` AND occurred_at >= ?`
		args = append(args, f.From.UTC().UnixMicro())
	}
	if !f.To.IsZero() {
		q += ` AND occurred_at <= ?`
		args = append(args, f.To.UTC().UnixMicro())
	}
	q += ` ORDER BY occurred_at DESC, id DESC LIMIT ?`
	args = append(args, f.Limit)
	return s.queryEvents(ctx, q, args...)
}

// ListSince returns events strictly after c, oldest created_at first.
// The zero cursor starts at the beginning.
func (s *Store) ListSince(ctx context.Context, c Cursor, limit int) ([]event.Event, error) {
	if err := checkLimit(limit); err != nil {
		return nil, err
	}
	flag := 0
	var micros int64
	id := ""
	switch {
	case c.ID == "" && c.CreatedAt.IsZero():
	case c.ID == "" || c.CreatedAt.IsZero():
		return nil, fmt.Errorf("invalid cursor")
	default:
		flag = 1
		micros = c.CreatedAt.UTC().UnixMicro()
		id = c.ID
	}
	q := `
		SELECT id, type, source_id, occurred_at, created_at, summary_nonce, summary_ct
		FROM events
		WHERE (? = 0 OR created_at > ? OR (created_at = ? AND id > ?))
		ORDER BY created_at ASC, id ASC
		LIMIT ?`
	return s.queryEvents(ctx, q, flag, micros, micros, id, limit)
}

// InsertNudge appends a delivery record. Detail is stored in the clear and
// must already be free of title and message text.
func (s *Store) InsertNudge(ctx context.Context, n Nudge) error {
	if err := validateNudge(n); err != nil {
		return err
	}
	n.CreatedAt = n.CreatedAt.UTC().Truncate(time.Microsecond)
	titleNonce, titleCT, err := s.box.Seal(crypt.AAD(n.ID, "title"), []byte(n.Title))
	if err != nil {
		return fmt.Errorf("seal nudge %s: %w", n.ID, err)
	}
	msgNonce, msgCT, err := s.box.Seal(crypt.AAD(n.ID, "message"), []byte(n.Message))
	if err != nil {
		return fmt.Errorf("seal nudge %s: %w", n.ID, err)
	}
	delivered := 0
	if n.Delivered {
		delivered = 1
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO nudges (
			id, created_at, channel, delivered, detail,
			title_nonce, title_ct, message_nonce, message_ct
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID,
		n.CreatedAt.UnixMicro(),
		n.Channel,
		delivered,
		n.Detail,
		titleNonce,
		titleCT,
		msgNonce,
		msgCT,
	)
	if err != nil {
		return fmt.Errorf("insert nudge %s: %w", n.ID, err)
	}
	s.log.DebugContext(ctx, "nudge recorded", "nudge_id", n.ID, "channel", n.Channel, "delivered", n.Delivered)
	return nil
}

func validateNudge(n Nudge) error {
	if !event.ValidID(n.ID) {
		return fmt.Errorf("invalid nudge id")
	}
	switch n.Channel {
	case "log", "ntfy":
	default:
		return fmt.Errorf("invalid nudge channel")
	}
	if strings.TrimSpace(n.Title) == "" || strings.TrimSpace(n.Message) == "" {
		return fmt.Errorf("nudge text is empty")
	}
	if len(n.Detail) > 200 {
		return fmt.Errorf("nudge detail is too long")
	}
	if n.CreatedAt.IsZero() {
		return fmt.Errorf("nudge created_at is empty")
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func (s *Store) queryEvents(ctx context.Context, q string, args ...any) ([]event.Event, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()
	out := []event.Event{}
	for rows.Next() {
		ev, err := s.scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	return out, nil
}

func (s *Store) scanEvent(sc scanner) (event.Event, error) {
	var ev event.Event
	var typ string
	var occurred, created int64
	var nonce, ct []byte
	if err := sc.Scan(&ev.ID, &typ, &ev.SourceID, &occurred, &created, &nonce, &ct); err != nil {
		return event.Event{}, fmt.Errorf("scan event: %w", err)
	}
	ev.Type = event.Type(typ)
	ev.OccurredAt = time.UnixMicro(occurred).UTC()
	ev.CreatedAt = time.UnixMicro(created).UTC()
	plain, err := s.box.Open(crypt.AAD(ev.ID, "summary"), nonce, ct)
	if err != nil {
		return event.Event{}, fmt.Errorf("read event %s: %w", ev.ID, err)
	}
	ev.Summary = string(plain)
	if err := ev.Validate(); err != nil {
		return event.Event{}, fmt.Errorf("stored event %s: %w", ev.ID, err)
	}
	return ev, nil
}

func checkLimit(limit int) error {
	if limit <= 0 || limit > MaxLimit {
		return fmt.Errorf("invalid limit")
	}
	return nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read store version: %w", err)
	}
	switch version {
	case 0:
		if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
			return fmt.Errorf("create store schema: %w", err)
		}
		if _, err := db.ExecContext(ctx, "PRAGMA user_version = 1"); err != nil {
			return fmt.Errorf("set store version: %w", err)
		}
	case 1:
	default:
		return fmt.Errorf("unsupported store version %d", version)
	}
	return nil
}

func sqliteDSN(path string) string {
	u := url.URL{
		Scheme:   "file",
		Path:     path,
		RawQuery: "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)",
	}
	return u.String()
}
