package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenbenja/daywire/internal/crypt"
)

const keyFileName = "data.key"

// Options overrides environment values. Empty fields fall through to the environment.
type Options struct {
	// DataDir is the directory for the database and the generated data key.
	DataDir string
	// Addr is the MCP listen address. It must be loopback before the server starts.
	Addr string
}

// Config is the resolved Week 1 configuration.
type Config struct {
	DataDir   string
	Addr      string
	NtfyURL   string
	NtfyToken string
	DataKey   []byte
	// KeySource is "env", "file", or "generated". It is safe to log.
	KeySource string
}

// DBPath returns the SQLite file path inside the data directory.
func (c Config) DBPath() string {
	return filepath.Join(c.DataDir, "events.db")
}

// Load resolves paths and the data key. It creates the data directory at mode 0700.
//
// Key precedence is DAYWIRE_DATA_KEY, then DAYWIRE_DATA_KEY_FILE, then
// <data dir>/data.key. A missing default key file is created with mode 0600.
// An explicit key file that does not exist is an error. The environment key is
// not written to disk.
func Load(opts Options) (Config, error) {
	addr := first(opts.Addr, os.Getenv("DAYWIRE_ADDR"), "127.0.0.1:8765")
	dataDir := first(opts.DataDir, os.Getenv("DAYWIRE_DATA_DIR"))
	if dataDir == "" {
		var err error
		dataDir, err = defaultDataDir()
		if err != nil {
			return Config{}, err
		}
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return Config{}, fmt.Errorf("create data dir: %w", err)
	}
	if err := os.Chmod(dataDir, 0o700); err != nil {
		return Config{}, fmt.Errorf("restrict data dir: %w", err)
	}
	key, source, err := loadKey(dataDir)
	if err != nil {
		return Config{}, err
	}
	return Config{
		DataDir:   dataDir,
		Addr:      addr,
		NtfyURL:   strings.TrimSpace(os.Getenv("DAYWIRE_NTFY_URL")),
		NtfyToken: strings.TrimSpace(os.Getenv("DAYWIRE_NTFY_TOKEN")),
		DataKey:   key,
		KeySource: source,
	}, nil
}

// ValidateLoopback reports whether addr is a host:port on the loopback interface.
// localhost and 127.0.0.0/8 and ::1 are accepted. Empty hosts and wildcards are not.
func ValidateLoopback(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("parse listen address: %w", err)
	}
	if port == "" {
		return fmt.Errorf("listen address %q has no port", addr)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("listen address %q is not loopback", addr)
	}
	return nil
}

func defaultDataDir() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "daywire"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "daywire"), nil
}

func loadKey(dataDir string) ([]byte, string, error) {
	if raw := strings.TrimSpace(os.Getenv("DAYWIRE_DATA_KEY")); raw != "" {
		key, err := parseKey([]byte(raw))
		if err != nil {
			return nil, "", fmt.Errorf("DAYWIRE_DATA_KEY: %w", err)
		}
		return key, "env", nil
	}
	explicit := strings.TrimSpace(os.Getenv("DAYWIRE_DATA_KEY_FILE"))
	path := explicit
	if path == "" {
		path = filepath.Join(dataDir, keyFileName)
	}
	body, err := os.ReadFile(path)
	if err == nil {
		key, perr := parseKey(body)
		if perr != nil {
			return nil, "", fmt.Errorf("read data key: %w", perr)
		}
		return key, "file", nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, "", fmt.Errorf("read data key: %w", err)
	}
	if explicit != "" {
		return nil, "", fmt.Errorf("data key file not found")
	}
	key, err := crypt.GenerateKey()
	if err != nil {
		return nil, "", err
	}
	encoded := []byte(base64.StdEncoding.EncodeToString(key) + "\n")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, "", fmt.Errorf("write data key: %w", err)
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil, "", fmt.Errorf("read data key: %w", rerr)
		}
		key, rerr = parseKey(body)
		if rerr != nil {
			return nil, "", fmt.Errorf("read data key: %w", rerr)
		}
		return key, "file", nil
	}
	if _, err := f.Write(encoded); err != nil {
		_ = f.Close()
		return nil, "", fmt.Errorf("write data key: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, "", fmt.Errorf("write data key: %w", err)
	}
	return key, "generated", nil
}

func parseKey(body []byte) ([]byte, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil, fmt.Errorf("data key is empty")
	}
	if dec, ok := decodeBase64Key(trimmed); ok {
		return dec, nil
	}
	if len(trimmed) == crypt.KeySize {
		out := make([]byte, crypt.KeySize)
		copy(out, trimmed)
		return out, nil
	}
	return nil, fmt.Errorf("data key must be 32 raw bytes or standard base64 of 32 bytes")
}

func decodeBase64Key(s string) ([]byte, bool) {
	dec, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(dec) != crypt.KeySize {
		var rawErr error
		dec, rawErr = base64.RawStdEncoding.DecodeString(s)
		if rawErr != nil || len(dec) != crypt.KeySize {
			return nil, false
		}
	}
	return dec, true
}

func first(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
