package mobile

import (
	"context"

	"github.com/chenbenja/daywire/internal/event"
	"github.com/chenbenja/daywire/internal/speech"
	"github.com/chenbenja/daywire/internal/voiceprint"
)

// SaveOwnerVoiceprint encrypts vec and writes speakers/owner.emb next to the
// database. A later call with the same phone replaces the file. The vector
// is not logged.
func (c *Core) SaveOwnerVoiceprint(vec []float32) error {
	if err := c.ready(); err != nil {
		return err
	}
	err := c.with(func(ctx context.Context) error {
		return voiceprint.Save(ctx, c.dataDir, c.key, voiceprint.OwnerID, vec)
	})
	if err != nil {
		return err
	}
	c.log.Info("owner voiceprint stored")
	return nil
}

// ForgetOwnerVoiceprint deletes the enrolled embedding. It succeeds when no
// embedding is stored, so the control can be tapped twice.
func (c *Core) ForgetOwnerVoiceprint() error {
	if err := c.ready(); err != nil {
		return err
	}
	err := c.with(func(ctx context.Context) error {
		return voiceprint.Forget(ctx, c.dataDir, voiceprint.OwnerID)
	})
	if err != nil {
		return err
	}
	c.log.Info("owner voiceprint forgotten")
	return nil
}

// OwnerEnrolled reports whether speakers/owner.emb exists. It does not decrypt
// the vector.
func (c *Core) OwnerEnrolled() (bool, error) {
	if err := c.ready(); err != nil {
		return false, err
	}
	var enrolled bool
	err := c.with(func(ctx context.Context) error {
		var qerr error
		enrolled, qerr = voiceprint.Exists(ctx, c.dataDir, voiceprint.OwnerID)
		return qerr
	})
	if err != nil {
		return false, err
	}
	return enrolled, nil
}

// LabelOwner scores vec against the enrolled voice. The result is owner,
// other, or unknown. The score and the vector are not logged.
func (c *Core) LabelOwner(vec []float32) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	var label string
	err := c.with(func(ctx context.Context) error {
		var qerr error
		label, qerr = voiceprint.Match(ctx, c.dataDir, c.key, voiceprint.OwnerID, vec)
		return qerr
	})
	if err != nil {
		return "", err
	}
	return label, nil
}

// InsertSpeechTurn stores one speaker turn as a note. language is en or he.
// label is owner, other, or unknown. text is the transcript. Control
// characters are stripped and the summary is capped. The source id is
// android:speech. This method does not take a file path, audio, or an embedding.
func (c *Core) InsertSpeechTurn(language, label, text string, occurredUnixMilli int64) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	summary, err := speech.Summary(language, label, text)
	if err != nil {
		return "", err
	}
	id, err := c.insert(event.TypeNote, speech.SourceID, summary, occurredUnixMilli)
	if err != nil {
		return "", err
	}
	c.log.Info("speech turn stored", "event_id", id, "language", language, "label", label)
	return id, nil
}
