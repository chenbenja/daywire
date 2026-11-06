package event

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Type is a semantic event kind.
type Type string

const (
	// TypeCommitment is something the user said they would do.
	TypeCommitment Type = "commitment"
	// TypeAsk is a request aimed at the user.
	TypeAsk Type = "ask"
	// TypeVisit is an arrival or place observation.
	TypeVisit Type = "visit"
	// TypeNote is a short redacted reminder or observation.
	TypeNote Type = "note"
	// TypeSpeech is one on-device speaker turn. The phone UI can show it.
	// Assistants cannot: query_events and subscribe omit this type. The
	// summary is capped transcript text, not a redacted semantic fact.
	TypeSpeech Type = "speech"
)

// MaxSummaryBytes is the largest redacted summary the store accepts.
const MaxSummaryBytes = 1024

// Event is one redacted semantic fact.
//
// Summary for commitment, ask, visit, and note is safe to show to an
// assistant. A speech summary is capped transcript text for the phone UI
// only. It is not call audio. SourceID names the producer (for example
// synthetic:calendar or android:speech), not a file path.
type Event struct {
	ID         string
	Type       Type
	SourceID   string
	OccurredAt time.Time
	CreatedAt  time.Time
	Summary    string
}

// ValidID reports whether id uses the Daywire identifier alphabet.
func ValidID(id string) bool {
	return validToken(id, 64)
}

// Valid reports whether the store accepts t.
func (t Type) Valid() bool {
	return t.AssistantVisible() || t == TypeSpeech
}

// AssistantVisible reports whether query_events and subscribe may return t.
// Speech turns are stored for the phone and are not assistant-visible.
func (t Type) AssistantVisible() bool {
	switch t {
	case TypeCommitment, TypeAsk, TypeVisit, TypeNote:
		return true
	default:
		return false
	}
}

// NewID returns a random UUID-shaped identifier.
func NewID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	), nil
}

// Validate checks required fields. Errors do not quote Summary.
func (e Event) Validate() error {
	if !validToken(e.ID, 64) {
		return fmt.Errorf("invalid id")
	}
	if !e.Type.Valid() {
		return fmt.Errorf("invalid event type")
	}
	if !validToken(e.SourceID, 128) {
		return fmt.Errorf("invalid source id")
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("occurred_at is empty")
	}
	if e.CreatedAt.IsZero() {
		return fmt.Errorf("created_at is empty")
	}
	if strings.TrimSpace(e.Summary) == "" {
		return fmt.Errorf("summary is empty")
	}
	if strings.ContainsRune(e.Summary, 0) || !utf8.ValidString(e.Summary) {
		return fmt.Errorf("summary is invalid")
	}
	if len(e.Summary) > MaxSummaryBytes {
		return fmt.Errorf("summary is too long")
	}
	return nil
}

// Normalize returns a copy with timestamps in UTC truncated to microseconds,
// which is the precision the store keeps.
func (e Event) Normalize() Event {
	e.OccurredAt = e.OccurredAt.UTC().Truncate(time.Microsecond)
	e.CreatedAt = e.CreatedAt.UTC().Truncate(time.Microsecond)
	return e
}

func validToken(s string, max int) bool {
	if s == "" || len(s) > max {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == ':' || r == '.':
		default:
			return false
		}
	}
	return true
}
