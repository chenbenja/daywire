package mobile

import "github.com/chenbenja/daywire/internal/mobile"

// Core is the on-device store handle passed to Kotlin.
type Core struct {
	inner *mobile.Core
}

// Open opens or creates the store at dataDir. Summaries and nudge text are
// encrypted; the database file is not. It does not listen on a socket and it
// does not contact ntfy.
func Open(dataDir string) (*Core, error) {
	inner, err := mobile.Open(dataDir)
	if err != nil {
		return nil, err
	}
	return &Core{inner: inner}, nil
}

// Close closes the database.
func (c *Core) Close() error {
	if c == nil || c.inner == nil {
		return nil
	}
	err := c.inner.Close()
	c.inner = nil
	return err
}

// InsertRedactedNote stores a note. summary must already be redacted. The
// Android UI may pass only the fixed synthetic sentence. Notification capture
// uses InsertNotification.
func (c *Core) InsertRedactedNote(sourceID, summary string, occurredUnixMilli int64) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	return c.inner.InsertRedactedNote(sourceID, summary, occurredUnixMilli)
}

// InsertNotification stores a note for an allowed notification. It does not
// take the notification title or body. category is the platform category
// constant, or an empty string.
func (c *Core) InsertNotification(packageName, appLabel, category string, postedUnixMilli int64) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	return c.inner.InsertNotification(packageName, appLabel, category, postedUnixMilli)
}

// InsertCoarseVisit stores a visit. The coordinates are quantized to a 0.01
// degree grid before they are stored. They are not logged.
func (c *Core) InsertCoarseVisit(latitude, longitude float64, occurredUnixMilli int64) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	return c.inner.InsertCoarseVisit(latitude, longitude, occurredUnixMilli)
}

// ListEventsJSON returns a JSON array of redacted events, newest first.
// eventType empty matches every type. limit 0 means 50.
func (c *Core) ListEventsJSON(eventType string, limit int) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	return c.inner.ListEventsJSON(eventType, limit)
}

// SaveOwnerVoiceprint encrypts an embedding into speakers/owner.emb.
// packed is little-endian float32 bits. The vector is not logged. A later
// call replaces the file.
func (c *Core) SaveOwnerVoiceprint(packed []byte) error {
	if err := c.ready(); err != nil {
		return err
	}
	return c.inner.SaveOwnerVoiceprint(packed)
}

// ForgetOwnerVoiceprint deletes the enrolled embedding. Missing is success.
func (c *Core) ForgetOwnerVoiceprint() error {
	if err := c.ready(); err != nil {
		return err
	}
	return c.inner.ForgetOwnerVoiceprint()
}

// OwnerEnrolled reports whether an owner embedding is stored.
func (c *Core) OwnerEnrolled() (bool, error) {
	if err := c.ready(); err != nil {
		return false, err
	}
	return c.inner.OwnerEnrolled()
}

// LabelOwner returns owner, other, or unknown for packed little-endian
// float32 bits. The score is not returned.
func (c *Core) LabelOwner(packed []byte) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	return c.inner.LabelOwner(packed)
}

// InsertSpeechTurn stores one speaker-labeled transcript as a speech event.
// It is not a note. language is en or he. label is owner, other, or unknown.
// It does not take a file path or audio.
func (c *Core) InsertSpeechTurn(language, label, text string, occurredUnixMilli int64) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	return c.inner.InsertSpeechTurn(language, label, text, occurredUnixMilli)
}

// RecordLocalNudge stores a nudge shown by the phone's own notification.
// It does not contact ntfy. delivered is false when the notification was not shown.
func (c *Core) RecordLocalNudge(title, message string, delivered bool) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	return c.inner.RecordLocalNudge(title, message, delivered)
}

func (c *Core) ready() error {
	if c == nil || c.inner == nil {
		return errClosed
	}
	return nil
}

type closedError struct{}

func (closedError) Error() string { return "core is closed" }

var errClosed error = closedError{}
