package mobile_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/testutil"
	"github.com/chenbenja/daywire/mobile"
)

func TestBindSurfaceRoundTrip(t *testing.T) {
	testutil.ClearEnv(t)
	core, err := mobile.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, core.Close()) })

	noteID, err := core.InsertRedactedNote("android:synthetic", "Note: synthetic check from the Daywire shell.", time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC).UnixMilli())
	require.NoError(t, err)
	mailID, err := core.InsertNotification("com.example.mail", "Example Mail", "email", time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC).UnixMilli())
	require.NoError(t, err)
	visitID, err := core.InsertCoarseVisit(32.08123456789, 34.78111999, time.Date(2026, 11, 6, 11, 0, 0, 0, time.UTC).UnixMilli())
	require.NoError(t, err)
	nudgeID, err := core.RecordLocalNudge("Synthetic nudge", "Pack the spare charger.", true)
	require.NoError(t, err)
	require.NotEmpty(t, nudgeID)

	raw, err := core.ListEventsJSON("", 10)
	require.NoError(t, err)
	var events []struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		SourceID string `json:"source_id"`
		Summary  string `json:"summary"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &events))
	require.Len(t, events, 3)
	require.Equal(t, []string{visitID, mailID, noteID}, []string{events[0].ID, events[1].ID, events[2].ID})
	require.Equal(t, "visit", events[0].Type)
	require.Equal(t, "Visit: coarse grid 32.08,34.78", events[0].Summary)
	require.NotContains(t, events[0].Summary, "08123456789")
	require.Equal(t, "note", events[1].Type)
	require.Equal(t, "android:notification:com.example.mail", events[1].SourceID)
	require.Equal(t, "Notification: Example Mail (com.example.mail), email", events[1].Summary)
	require.Equal(t, "android:synthetic", events[2].SourceID)

	require.NoError(t, core.SaveOwnerVoiceprint([]float32{1, 0}))
	enrolled, err := core.OwnerEnrolled()
	require.NoError(t, err)
	require.True(t, enrolled)
	label, err := core.LabelOwner([]float32{0, 1})
	require.NoError(t, err)
	require.Equal(t, "other", label)
	speechID, err := core.InsertSpeechTurn("he", "other", "synthetic turn", time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC).UnixMilli())
	require.NoError(t, err)
	require.NotEmpty(t, speechID)
	require.NoError(t, core.ForgetOwnerVoiceprint())
	enrolled, err = core.OwnerEnrolled()
	require.NoError(t, err)
	require.False(t, enrolled)

	require.NoError(t, core.Close())
	_, err = core.ListEventsJSON("", 10)
	require.Error(t, err)
}
