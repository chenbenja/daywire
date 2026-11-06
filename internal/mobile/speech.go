package mobile

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/chenbenja/daywire/internal/event"
	"github.com/chenbenja/daywire/internal/speech"
	"github.com/chenbenja/daywire/internal/voiceprint"
)

// SaveOwnerVoiceprint encrypts the embedding and writes speakers/owner.emb
// next to the database. packed is little-endian float32 bits. A later call
// replaces the file. The vector is not logged. The parameter is []byte
// because gobind does not generate methods that take []float32.
func (c *Core) SaveOwnerVoiceprint(packed []byte) error {
	if err := c.ready(); err != nil {
		return err
	}
	vec, err := unpackEmbedding(packed)
	if err != nil {
		return err
	}
	err = c.with(func(ctx context.Context) error {
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

// LabelOwner scores packed against the enrolled voice. packed is
// little-endian float32 bits. The result is owner, other, or unknown.
// The score and the vector are not logged.
func (c *Core) LabelOwner(packed []byte) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	vec, err := unpackEmbedding(packed)
	if err != nil {
		return "", err
	}
	var label string
	err = c.with(func(ctx context.Context) error {
		var qerr error
		label, qerr = voiceprint.Match(ctx, c.dataDir, c.key, voiceprint.OwnerID, vec)
		return qerr
	})
	if err != nil {
		return "", err
	}
	return label, nil
}

// InsertSpeechTurn stores one speaker turn as a speech event, not a note.
// language is en or he. label is owner, other, or unknown. text is the
// transcript. Control characters are stripped and the summary is capped.
// The source id is android:speech. This method does not take a file path,
// audio, or an embedding. query_events and subscribe do not return the row.
func (c *Core) InsertSpeechTurn(language, label, text string, occurredUnixMilli int64) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	summary, err := speech.Summary(language, label, text)
	if err != nil {
		return "", err
	}
	id, err := c.insert(event.TypeSpeech, speech.SourceID, summary, occurredUnixMilli)
	if err != nil {
		return "", err
	}
	c.log.Info("speech turn stored", "event_id", id, "language", language, "label", label)
	return id, nil
}

// unpackEmbedding reads little-endian float32 bits. The error does not quote
// the bytes.
func unpackEmbedding(packed []byte) ([]float32, error) {
	if len(packed) == 0 || len(packed)%4 != 0 || len(packed) > 4096*4 {
		return nil, fmt.Errorf("invalid embedding")
	}
	out := make([]float32, len(packed)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(packed[i*4:]))
	}
	return out, nil
}
