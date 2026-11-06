package mobile_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMobileDepsExcludeSpeechAndServer(t *testing.T) {
	t.Parallel()
	for _, pkg := range []string{
		"github.com/chenbenja/daywire/mobile",
		"github.com/chenbenja/daywire/internal/mobile",
	} {
		t.Run(pkg, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", pkg)
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))
			for _, dep := range strings.Split(string(out), "\n") {
				dep = strings.TrimSpace(dep)
				if dep == "" {
					continue
				}
				require.NotContains(t, dep, "sherpa")
				require.NotContains(t, dep, "onnx")
				require.NotContains(t, dep, "internal/speaker")
				require.NotContains(t, dep, "internal/server")
				require.NotContains(t, dep, "internal/cli")
				require.NotContains(t, dep, "internal/nudge")
				require.NotContains(t, dep, "net/http")
			}
		})
	}
}
