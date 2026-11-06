package speaker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindDaywireRootIgnoresOtherModules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	other := filepath.Join(root, "other", "nested")
	require.NoError(t, os.MkdirAll(other, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "other", "go.mod"), []byte("module example.com/other\n\ngo 1.26.0\n"), 0o644))
	_, err := findDaywireRoot(other)
	require.Error(t, err)

	dw := filepath.Join(root, "daywire", "internal", "speaker")
	require.NoError(t, os.MkdirAll(dw, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "daywire", "go.mod"), []byte("// comment\nmodule github.com/chenbenja/daywire\n"), 0o644))
	got, err := findDaywireRoot(dw)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "daywire"), got)
}
