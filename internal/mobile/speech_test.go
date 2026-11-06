package mobile_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/mobile"
	"github.com/chenbenja/daywire/internal/testutil"
)

func TestSpeechTurnRedactsAndLabels(t *testing.T) {
	testutil.ClearEnv(t)
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	dir := t.TempDir()
	core, err := mobile.OpenWithLogger(dir, logger)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, core.Close()) })

	const spoken = "Jane Doe left the spare key"
	enrolled, err := core.OwnerEnrolled()
	require.NoError(t, err)
	require.False(t, enrolled)

	vec := []float32{1, 0, 0, 0}
	require.NoError(t, core.SaveOwnerVoiceprint(packFloats(vec)))
	enrolled, err = core.OwnerEnrolled()
	require.NoError(t, err)
	require.True(t, enrolled)

	label, err := core.LabelOwner(packFloats([]float32{0.9, 0.1, 0, 0}))
	require.NoError(t, err)
	require.Equal(t, "owner", label)
	label, err = core.LabelOwner(packFloats([]float32{0, 1, 0, 0}))
	require.NoError(t, err)
	require.Equal(t, "other", label)
	label, err = core.LabelOwner(packFloats([]float32{0.37, 0.929, 0, 0}))
	require.NoError(t, err)
	require.Equal(t, "unknown", label)
	_, err = core.LabelOwner([]byte{1, 2, 3})
	require.EqualError(t, err, "invalid embedding")

	when := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	_, err = core.InsertRedactedNote("android:synthetic", "Note: synthetic check from the Daywire shell.", when.Add(-time.Minute).UnixMilli())
	require.NoError(t, err)
	id, err := core.InsertSpeechTurn("en", "owner", spoken+"\nrest", when.UnixMilli())
	require.NoError(t, err)

	notes, err := core.ListEventsJSON("note", 10)
	require.NoError(t, err)
	require.NotContains(t, notes, spoken)
	require.NotContains(t, notes, id)
	require.Contains(t, notes, "android:synthetic")

	raw, err := core.ListEventsJSON("speech", 10)
	require.NoError(t, err)
	var events []struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		SourceID string `json:"source_id"`
		Summary  string `json:"summary"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &events))
	require.Len(t, events, 1)
	require.Equal(t, id, events[0].ID)
	require.Equal(t, "speech", events[0].Type)
	require.Equal(t, "android:speech", events[0].SourceID)
	require.Equal(t, "en owner: "+spoken+"rest", events[0].Summary)
	require.NotContains(t, events[0].Summary, "\n")
	require.NotContains(t, events[0].SourceID, ".wav")

	all, err := core.ListEventsJSON("", 10)
	require.NoError(t, err)
	require.Contains(t, all, id)
	require.Contains(t, all, "android:synthetic")

	require.NoError(t, core.Checkpoint())
	blob := readStoreFiles(t, core.DBPath())
	require.NotContains(t, string(blob), spoken)
	require.NotContains(t, logs.String(), spoken)
	require.NotContains(t, logs.String(), "Jane")
	require.Contains(t, logs.String(), "speech turn stored")
	require.Contains(t, logs.String(), "language=en")
	require.Contains(t, logs.String(), "label=owner")

	emb, err := os.ReadFile(filepath.Join(dir, "speakers", "owner.emb"))
	require.NoError(t, err)
	rawVec := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(rawVec[i*4:], math.Float32bits(v))
	}
	require.False(t, bytes.Contains(emb, rawVec))
	st, err := os.Stat(filepath.Join(dir, "speakers", "owner.emb"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), st.Mode().Perm())

	require.NoError(t, core.ForgetOwnerVoiceprint())
	enrolled, err = core.OwnerEnrolled()
	require.NoError(t, err)
	require.False(t, enrolled)
	_, err = core.LabelOwner(packFloats(vec))
	require.Error(t, err)
	require.NotContains(t, err.Error(), spoken)
}

func packFloats(vec []float32) []byte {
	out := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(v))
	}
	return out
}

func TestSpeechTurnRejectsBadTextWithoutQuotingIt(t *testing.T) {
	t.Parallel()
	core := openCore(t)
	const secret = "do not quote this transcript"
	_, err := core.InsertSpeechTurn("en", "owner", " \n\x00 ", time.Now().UnixMilli())
	require.EqualError(t, err, "speech text is empty")
	_, err = core.InsertSpeechTurn("english", "owner", secret, time.Now().UnixMilli())
	require.EqualError(t, err, "invalid speech language")
	require.NotContains(t, err.Error(), secret)
	_, err = core.InsertSpeechTurn("he", "speaker0", secret, time.Now().UnixMilli())
	require.EqualError(t, err, "invalid speech label")
	require.NotContains(t, err.Error(), secret)
	require.NotContains(t, err.Error(), "/")

	id, err := core.InsertSpeechTurn("he", "unknown", strings.Repeat("א", 2000), time.Now().UnixMilli())
	require.NoError(t, err)
	require.NotEmpty(t, id)
}
