package speaker

import (
	"context"

	"github.com/chenbenja/daywire/internal/crypt"
	"github.com/chenbenja/daywire/internal/voiceprint"
)

// ErrNotEnrolled means this data directory has no profile for the id.
var ErrNotEnrolled = voiceprint.ErrNotEnrolled

// Profile is one enrolled voice. Vec is plaintext only in memory.
type Profile struct {
	ID  string
	Vec []float32
}

// ValidSpeakerID reports whether id is safe to use as a file name.
func ValidSpeakerID(id string) bool {
	return voiceprint.ValidID(id)
}

// SaveProfile encrypts vec and writes it under dataDir/speakers.
// The directory is mode 0700 and the file is mode 0600. A later save with
// the same id replaces the file.
func SaveProfile(ctx context.Context, dataDir string, key []byte, id string, vec []float32) error {
	return voiceprint.Save(ctx, dataDir, key, id, vec)
}

// LoadProfile decrypts the profile for id.
func LoadProfile(ctx context.Context, dataDir string, key []byte, id string) (Profile, error) {
	got, err := voiceprint.Load(ctx, dataDir, key, id)
	if err != nil {
		return Profile{}, err
	}
	return Profile{ID: got.ID, Vec: got.Vec}, nil
}

func profileAAD(id string) []byte {
	return crypt.AAD(id, "speaker.embedding")
}
