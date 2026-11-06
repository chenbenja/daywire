package cli_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/cli"
	"github.com/chenbenja/daywire/internal/speaker"
	"github.com/chenbenja/daywire/internal/testutil"
)

func TestSpeakerEnrollLabelAndDemo(t *testing.T) {
	testutil.ClearEnv(t)
	// Cold ONNX load under the race detector can exceed the shared 30s helper.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)

	data := t.TempDir()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	clip, err := speaker.LoadDemo(ctx, filepath.Join("..", "speaker", "testdata"))
	require.NoError(t, err)
	mix := filepath.Join(t.TempDir(), "mix.wav")
	require.NoError(t, speaker.WriteWAV(ctx, mix, clip.SampleRate, clip.Mixed))
	enroll := filepath.Join("..", "speaker", "testdata", speaker.FixtureEnroll)

	err = cli.Run(ctx, []string{
		"daywire", "speaker", "enroll",
		"--data-dir", data,
		"--wav", enroll,
	}, logger)
	if errors.Is(err, speaker.ErrEngineUnavailable) {
		require.ErrorIs(t, err, speaker.ErrEngineUnavailable)
		return
	}
	if errors.Is(err, speaker.ErrModelsUnavailable) {
		t.Skip(err.Error())
	}
	require.NoError(t, err)
	require.Contains(t, buf.String(), "enroll succeeded")
	require.Contains(t, buf.String(), "dim=256")

	profile := filepath.Join(data, "speakers", "owner.emb")
	st, err := os.Stat(profile)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	_, err = os.Stat(filepath.Join(data, "events.db"))
	require.ErrorIs(t, err, os.ErrNotExist)
	body, err := os.ReadFile(profile)
	require.NoError(t, err)
	require.NotContains(t, string(body), "RIFF")

	err = cli.Run(ctx, []string{
		"daywire", "speaker", "label",
		"--data-dir", data,
		"--wav", mix,
	}, logger)
	require.NoError(t, err)
	assertOwnerOnFirstSpeaker(t, buf.String(), clip)

	buf.Reset()
	demoDir := t.TempDir()
	err = cli.Run(ctx, []string{
		"daywire", "speaker", "demo",
		"--data-dir", demoDir,
	}, logger)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "enroll succeeded")
	assertOwnerOnFirstSpeaker(t, buf.String(), clip)
	_, err = os.Stat(filepath.Join(demoDir, "events.db"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func assertOwnerOnFirstSpeaker(t *testing.T, logs string, clip speaker.DemoClip) {
	t.Helper()
	re := regexp.MustCompile(`speaker turn.*start=([0-9.]+) end=([0-9.]+) label=([a-z]+)`)
	matches := re.FindAllStringSubmatch(logs, -1)
	require.NotEmpty(t, matches)
	var ownerN, otherN int
	cut := clip.OwnerEnd + (clip.OtherStart-clip.OwnerEnd)/2
	for _, m := range matches {
		start, err := strconv.ParseFloat(m[1], 64)
		require.NoError(t, err)
		end, err := strconv.ParseFloat(m[2], 64)
		require.NoError(t, err)
		mid := (start + end) / 2
		switch m[3] {
		case speaker.OwnerLabel:
			ownerN++
			require.Less(t, mid, cut)
		case speaker.OtherLabel:
			otherN++
			require.Greater(t, mid, cut)
		default:
			t.Fatalf("label %s", m[3])
		}
	}
	require.Greater(t, ownerN, 0)
	require.Greater(t, otherN, 0)
	require.Contains(t, logs, "owner labeled")
}
