package mobile_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/mobile"
	"github.com/chenbenja/daywire/internal/testutil"
)

const marker = "REDACTED-SUMMARY-MARKER-9f3c2a"

func TestNotificationSummary(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		label    string
		pkg      string
		category string
		want     string
		wantErr  bool
	}{
		{
			name:  "label and package",
			label: "Example Mail", pkg: "com.example.mail",
			want: "Notification: Example Mail (com.example.mail)",
		},
		{
			name:  "email category",
			label: "Example Mail", pkg: "com.example.mail", category: "email",
			want: "Notification: Example Mail (com.example.mail), email",
		},
		{
			name:  "message category uses a word",
			label: "Example Chat", pkg: "com.example.chat", category: "msg",
			want: "Notification: Example Chat (com.example.chat), message",
		},
		{
			name:  "unknown category is dropped",
			label: "Example Mail", pkg: "com.example.mail",
			category: "pick up milk from the desk",
			want:     "Notification: Example Mail (com.example.mail)",
		},
		{
			name:  "empty label falls back to the package",
			label: " \n\t", pkg: "com.example.mail",
			want: "Notification: com.example.mail (com.example.mail)",
		},
		{
			name:  "controls are stripped from the label",
			label: "Example\nMail", pkg: "com.example.mail",
			want: "Notification: ExampleMail (com.example.mail)",
		},
		{name: "empty package", label: "Mail", pkg: " ", wantErr: true},
		{name: "one segment", label: "Mail", pkg: "mail", wantErr: true},
		{name: "path", label: "Mail", pkg: "../com.example", wantErr: true},
		{name: "space", label: "Mail", pkg: "com.example.mail app", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := mobile.NotificationSummary(tc.label, tc.pkg, tc.category)
			if tc.wantErr {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "milk")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			require.NotContains(t, got, "milk")
			require.NotContains(t, got, "\n")
		})
	}
}

func TestNotificationLabelIsCapped(t *testing.T) {
	t.Parallel()
	label := strings.Repeat("a", 200)
	got, err := mobile.NotificationSummary(label, "com.example.mail", "")
	require.NoError(t, err)
	require.Contains(t, got, strings.Repeat("a", 80))
	require.NotContains(t, got, strings.Repeat("a", 81))
}

func TestCoarseVisitSummary(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		lat     float64
		lon     float64
		want    string
		wantErr bool
	}{
		{name: "quantize", lat: 32.08123456789, lon: 34.784, want: "Visit: coarse grid 32.08,34.78"},
		{name: "half away from zero", lat: 32.085, lon: 0, want: "Visit: coarse grid 32.09,0.00"},
		{name: "negative", lat: -33.333, lon: 151.209, want: "Visit: coarse grid -33.33,151.21"},
		{name: "nan", lat: math.NaN(), lon: 0, wantErr: true},
		{name: "inf", lat: 0, lon: math.Inf(1), wantErr: true},
		{name: "lat range", lat: 91, lon: 0, wantErr: true},
		{name: "lon range", lat: 0, lon: -181, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := mobile.CoarseVisitSummary(tc.lat, tc.lon)
			if tc.wantErr {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "91")
				require.NotContains(t, err.Error(), "181")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			require.NotContains(t, got, "08123456789")
			require.NotContains(t, got, "34.784")
		})
	}
}

func TestInsertReadSyntheticEventAndLocalNudge(t *testing.T) {
	testutil.ClearEnv(t)
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	core, err := mobile.OpenWithLogger(t.TempDir(), logger)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, core.Close()) })

	when := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	id, err := core.InsertRedactedNote("android:synthetic", "note "+marker, when.UnixMilli())
	require.NoError(t, err)
	require.NotEmpty(t, id)

	raw, err := core.ListEventsJSON("note", 10)
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
	require.Equal(t, "note", events[0].Type)
	require.Equal(t, "android:synthetic", events[0].SourceID)
	require.Equal(t, "note "+marker, events[0].Summary)

	nudgeID, err := core.RecordLocalNudge("Synthetic nudge", "nudge "+marker, true)
	require.NoError(t, err)
	nudgeRaw, err := core.NudgeJSON(nudgeID)
	require.NoError(t, err)
	var nudge struct {
		ID        string `json:"id"`
		Channel   string `json:"channel"`
		Delivered bool   `json:"delivered"`
		Detail    string `json:"detail"`
		Title     string `json:"title"`
		Message   string `json:"message"`
	}
	require.NoError(t, json.Unmarshal([]byte(nudgeRaw), &nudge))
	require.Equal(t, nudgeID, nudge.ID)
	require.Equal(t, "local", nudge.Channel)
	require.True(t, nudge.Delivered)
	require.Equal(t, "shown", nudge.Detail)
	require.Equal(t, "Synthetic nudge", nudge.Title)
	require.Equal(t, "nudge "+marker, nudge.Message)

	require.NoError(t, core.Checkpoint())
	blob := readStoreFiles(t, core.DBPath())
	require.NotContains(t, string(blob), marker)
	require.NotContains(t, string(blob), "Synthetic nudge")
	require.NotContains(t, logs.String(), marker)
	require.NotContains(t, logs.String(), "Synthetic nudge")
}

func TestNotificationAndVisitRoundTrip(t *testing.T) {
	testutil.ClearEnv(t)
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	core, err := mobile.OpenWithLogger(t.TempDir(), logger)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, core.Close()) })

	const (
		rawLat = 32.08123456789
		rawLon = 34.78111999
		body   = "pick up milk from the desk"
	)
	noteID, err := core.InsertNotification("com.example.mail", "Example Mail", body, time.Date(2026, 11, 6, 8, 0, 0, 0, time.UTC).UnixMilli())
	require.NoError(t, err)
	visitID, err := core.InsertCoarseVisit(rawLat, rawLon, time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC).UnixMilli())
	require.NoError(t, err)

	raw, err := core.ListEventsJSON("", 10)
	require.NoError(t, err)
	var events []struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		SourceID string `json:"source_id"`
		Summary  string `json:"summary"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &events))
	require.Len(t, events, 2)
	require.Equal(t, visitID, events[0].ID)
	require.Equal(t, "visit", events[0].Type)
	require.Equal(t, "android:location", events[0].SourceID)
	require.Equal(t, "Visit: coarse grid 32.08,34.78", events[0].Summary)
	require.Equal(t, noteID, events[1].ID)
	require.Equal(t, "note", events[1].Type)
	require.Equal(t, "android:notification:com.example.mail", events[1].SourceID)
	require.Equal(t, "Notification: Example Mail (com.example.mail)", events[1].Summary)
	require.NotContains(t, raw, body)
	require.NotContains(t, raw, "08123456789")
	require.NotContains(t, raw, "78111999")

	filtered, err := core.ListEventsJSON("visit", 0)
	require.NoError(t, err)
	require.Contains(t, filtered, visitID)
	require.NotContains(t, filtered, noteID)

	require.NotContains(t, logs.String(), "08123456789")
	require.NotContains(t, logs.String(), "78111999")
	require.NotContains(t, logs.String(), body)
	require.NotContains(t, logs.String(), "Example Mail")
}

func TestRecordLocalNudgeWithoutDelivery(t *testing.T) {
	testutil.ClearEnv(t)
	core := openCore(t)
	id, err := core.RecordLocalNudge("Synthetic nudge", "Stored only.", false)
	require.NoError(t, err)
	raw, err := core.NudgeJSON(id)
	require.NoError(t, err)
	require.Contains(t, raw, `"channel":"local"`)
	require.Contains(t, raw, `"delivered":false`)
	require.Contains(t, raw, `"detail":"not shown"`)

	_, err = core.RecordLocalNudge("  ", "Stored only.", true)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "Stored only.")
}

func TestOpenRejectsEmptyDir(t *testing.T) {
	t.Parallel()
	_, err := mobile.Open("  ")
	require.Error(t, err)
}

func TestClosedCore(t *testing.T) {
	testutil.ClearEnv(t)
	core := openCore(t)
	require.NoError(t, core.Close())
	_, err := core.ListEventsJSON("", 10)
	require.Error(t, err)
	_, err = core.InsertRedactedNote("android:synthetic", "note", time.Now().UnixMilli())
	require.Error(t, err)
}

func TestInvalidEventType(t *testing.T) {
	testutil.ClearEnv(t)
	core := openCore(t)
	_, err := core.ListEventsJSON("audio", 10)
	require.Error(t, err)
}

func TestCoreDoesNotCallNtfy(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("core.go")
	require.NoError(t, err)
	src := string(body)
	require.NotContains(t, src, "nudge.New")
	require.NotContains(t, src, ".Deliver(")
	require.NotContains(t, src, "net/http")
	require.NotContains(t, src, "DAYWIRE_NTFY")
	require.NotContains(t, src, "ListenAndServe")
}

func openCore(t *testing.T) *mobile.Core {
	t.Helper()
	core, err := mobile.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, core.Close()) })
	return core
}

func readStoreFiles(t *testing.T, path string) []byte {
	t.Helper()
	var blob []byte
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		b, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		require.NoError(t, err)
		blob = append(blob, b...)
	}
	return blob
}
