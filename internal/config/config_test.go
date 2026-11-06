package config_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/config"
	"github.com/chenbenja/daywire/internal/crypt"
	"github.com/chenbenja/daywire/internal/testutil"
)

func TestLoadGeneratesKeyUnderXDG(t *testing.T) {
	testutil.ClearEnv(t)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	cfg, err := config.Load(config.Options{})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(xdg, "daywire"), cfg.DataDir)
	require.Equal(t, "127.0.0.1:8765", cfg.Addr)
	require.Equal(t, "generated", cfg.KeySource)
	require.Len(t, cfg.DataKey, crypt.KeySize)

	info, err := os.Stat(filepath.Join(cfg.DataDir, "data.key"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	dirInfo, err := os.Stat(cfg.DataDir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())

	again, err := config.Load(config.Options{})
	require.NoError(t, err)
	require.Equal(t, cfg.DataKey, again.DataKey)
	require.Equal(t, "file", again.KeySource)
}

func TestEnvKeyWinsAndIsNotWritten(t *testing.T) {
	testutil.ClearEnv(t)
	dir := t.TempDir()
	key, err := crypt.GenerateKey()
	require.NoError(t, err)
	t.Setenv("DAYWIRE_DATA_KEY", base64.StdEncoding.EncodeToString(key))

	cfg, err := config.Load(config.Options{DataDir: dir, Addr: "127.0.0.1:9"})
	require.NoError(t, err)
	require.Equal(t, key, cfg.DataKey)
	require.Equal(t, "env", cfg.KeySource)
	_, err = os.Stat(filepath.Join(dir, "data.key"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestRejectsShortKeyWithoutEchoingIt(t *testing.T) {
	testutil.ClearEnv(t)
	t.Setenv("DAYWIRE_DATA_KEY", base64.StdEncoding.EncodeToString([]byte("short-secret")))
	_, err := config.Load(config.Options{DataDir: t.TempDir()})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "short-secret")
}

func TestMissingExplicitKeyFile(t *testing.T) {
	testutil.ClearEnv(t)
	t.Setenv("DAYWIRE_DATA_KEY_FILE", filepath.Join(t.TempDir(), "missing.key"))
	_, err := config.Load(config.Options{DataDir: t.TempDir()})
	require.ErrorContains(t, err, "data key file not found")
}

func TestValidateLoopback(t *testing.T) {
	t.Parallel()
	cases := []struct {
		addr string
		ok   bool
	}{
		{addr: "127.0.0.1:8765", ok: true},
		{addr: "127.0.0.2:1", ok: true},
		{addr: "localhost:8765", ok: true},
		{addr: "[::1]:8765", ok: true},
		{addr: "0.0.0.0:8765", ok: false},
		{addr: ":8765", ok: false},
		{addr: "192.168.1.5:8765", ok: false},
		{addr: "[::]:8765", ok: false},
		{addr: "not-an-addr", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.addr, func(t *testing.T) {
			t.Parallel()
			err := config.ValidateLoopback(tc.addr)
			if tc.ok {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}
