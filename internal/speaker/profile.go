package speaker

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenbenja/daywire/internal/crypt"
)

const (
	profileMagic   = "DWSP"
	profileVersion = 1
	gcmNonceSize   = 12
)

// ErrNotEnrolled means this data directory has no profile for the id.
var ErrNotEnrolled = errors.New("speaker is not enrolled")

// Profile is one enrolled voice. Vec is plaintext only in memory.
type Profile struct {
	ID  string
	Vec []float32
}

// ValidSpeakerID reports whether id is safe to use as a file name.
// Ids are lowercase so "owner" cannot collide with "Owner".
func ValidSpeakerID(id string) bool {
	if id == "" || len(id) > 32 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z':
		case i > 0 && c >= '0' && c <= '9':
		case i > 0 && (c == '_' || c == '-'):
		default:
			return false
		}
	}
	return true
}

// SaveProfile encrypts vec and writes it under dataDir/speakers.
// The directory is mode 0700 and the file is mode 0600. A later save with
// the same id replaces the file.
func SaveProfile(ctx context.Context, dataDir string, key []byte, id string, vec []float32) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	body, err := sealProfile(id, vec, key)
	if err != nil {
		return err
	}
	dir, path, err := profilePaths(dataDir, id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create speaker dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("restrict speaker dir: %w", err)
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("write speaker profile: %w", err)
	}
	_, werr := f.Write(body)
	cerr := f.Close()
	if werr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write speaker profile: %w", werr)
	}
	if cerr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write speaker profile: %w", cerr)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("restrict speaker profile: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write speaker profile: %w", err)
	}
	return nil
}

// LoadProfile decrypts the profile for id.
func LoadProfile(ctx context.Context, dataDir string, key []byte, id string) (Profile, error) {
	if err := ctx.Err(); err != nil {
		return Profile{}, err
	}
	_, path, err := profilePaths(dataDir, id)
	if err != nil {
		return Profile{}, err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Profile{}, ErrNotEnrolled
		}
		return Profile{}, fmt.Errorf("read speaker profile: %w", err)
	}
	vec, err := openProfile(id, body, key)
	if err != nil {
		return Profile{}, err
	}
	return Profile{ID: id, Vec: vec}, nil
}

func profilePaths(dataDir, id string) (dir, path string, err error) {
	if !ValidSpeakerID(id) {
		return "", "", fmt.Errorf("invalid speaker id")
	}
	if dataDir == "" {
		return "", "", fmt.Errorf("data dir is empty")
	}
	dir = filepath.Join(dataDir, "speakers")
	path = filepath.Join(dir, id+".emb")
	rel, err := filepath.Rel(dir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", "", fmt.Errorf("invalid speaker id")
	}
	return dir, path, nil
}

// profileAAD binds an embedding to the speaker spike.
//
// The event store seals with columns summary, title, and message. This column
// is speaker.embedding. Identifiers accepted by either package cannot contain
// a NUL, and AAD is id, a NUL, then the column, so the two ciphertexts cannot
// match.
func profileAAD(id string) []byte {
	return crypt.AAD(id, "speaker.embedding")
}

func sealProfile(id string, vec []float32, key []byte) ([]byte, error) {
	if err := checkEmbedding(vec); err != nil {
		return nil, err
	}
	box, err := crypt.NewBox(key)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(plain[i*4:], math.Float32bits(v))
	}
	nonce, ct, err := box.Seal(profileAAD(id), plain)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcmNonceSize {
		return nil, fmt.Errorf("unexpected nonce length")
	}
	var buf bytes.Buffer
	buf.Grow(4 + 1 + 4 + len(nonce) + len(ct))
	buf.WriteString(profileMagic)
	buf.WriteByte(profileVersion)
	var dim [4]byte
	binary.LittleEndian.PutUint32(dim[:], uint32(len(vec)))
	buf.Write(dim[:])
	buf.Write(nonce)
	buf.Write(ct)
	return buf.Bytes(), nil
}

func openProfile(id string, body, key []byte) ([]float32, error) {
	if len(body) < 4+1+4+gcmNonceSize+16 {
		return nil, fmt.Errorf("speaker profile is truncated")
	}
	if string(body[:4]) != profileMagic || body[4] != profileVersion {
		return nil, fmt.Errorf("speaker profile version is unsupported")
	}
	dim := binary.LittleEndian.Uint32(body[5:9])
	if dim == 0 || dim > 4096 {
		return nil, fmt.Errorf("speaker profile length is invalid")
	}
	nonce := body[9 : 9+gcmNonceSize]
	ct := body[9+gcmNonceSize:]
	box, err := crypt.NewBox(key)
	if err != nil {
		return nil, err
	}
	plain, err := box.Open(profileAAD(id), nonce, ct)
	if err != nil {
		return nil, fmt.Errorf("open speaker profile: %w", err)
	}
	if len(plain) != int(dim)*4 {
		return nil, fmt.Errorf("speaker profile length is invalid")
	}
	vec := make([]float32, dim)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(plain[i*4:]))
	}
	if err := checkEmbedding(vec); err != nil {
		return nil, err
	}
	return vec, nil
}
