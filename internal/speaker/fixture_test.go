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
		{name: FixtureEnroll, sha: "746c756a0b42872638c5f4bb5a8e282ca113bcdaf786bc54591d745dbed96f47"},
		{name: FixtureOwner, sha: "6d9736c06b09ac23c50a668047f430e157edf980e79629a65d0841cd195927a8"},
		{name: FixtureOther, sha: "1a50ccdc40469ef16df72473e4cf7502cf36afcb550b75d80d6f923ca866d955"},
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
