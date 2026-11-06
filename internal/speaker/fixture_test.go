package speaker

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFixtureChecksums(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		sha  string
	}{
		{name: "speaker1_a_en_16k.wav", sha: "cb35bff3dac9aec36e259461fecae1e1bc2ec029615f30713111cd598993676c"},
		{name: "speaker1_b_en_16k.wav", sha: "d7daff767e13d9a2187b676d958065121cd5e26da046d65cd9604e91a87525a2"},
		{name: "speaker2_a_en_16k.wav", sha: "a723c134978a17fe12ca2374d0281a8003a56fa44ff9d2249a08791714983362"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body, err := os.ReadFile(filepath.Join("testdata", tc.name))
			require.NoError(t, err)
			sum := sha256.Sum256(body)
			require.Equal(t, tc.sha, hex.EncodeToString(sum[:]))
		})
	}
}
