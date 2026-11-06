package mobile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAndroidShellPrivacyBoundary(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "android")
	info, err := os.Stat(root)
	require.NoError(t, err)
	require.True(t, info.IsDir())

	var kotlin []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == "build" || base == ".gradle" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".kt", ".kts", ".xml", ".pro", ".properties":
		default:
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(body)
		rel := strings.TrimPrefix(path, root+string(filepath.Separator))
		for _, forbidden := range []string{
			".extras",
			"EXTRA_",
			"tickerText",
			"RECORD_AUDIO",
			"ACCESS_FINE_LOCATION",
			"ACCESS_BACKGROUND_LOCATION",
			"android.permission.INTERNET",
			"sherpa",
			"onnx",
			"whisper",
			"moonshine",
			"telegram",
			"firebase",
			"openInputStream",
			"openFileDescriptor",
			"MediaRecorder",
			"AudioRecord",
			"SpeechRecognizer",
		} {
			require.NotContains(t, strings.ToLower(text), strings.ToLower(forbidden), "%s contains %s", rel, forbidden)
		}
		if ext == ".kt" {
			kotlin = append(kotlin, text)
			for i, line := range strings.Split(text, "\n") {
				if !strings.Contains(line, "Log.") && !strings.Contains(line, "Trace.") {
					continue
				}
				lower := strings.ToLower(line)
				for _, bad := range []string{"latitude", "longitude", "summary", "ticker", "message"} {
					require.NotContains(t, lower, bad, "%s:%d", rel, i+1)
				}
			}
		}
		return nil
	})
	require.NoError(t, err)
	joined := strings.Join(kotlin, "\n")
	require.Contains(t, joined, "insertNotification(")
	require.Contains(t, joined, "insertCoarseVisit(")
	require.Contains(t, joined, "recordLocalNudge(")
	require.Contains(t, joined, "OpenDocumentTree")
	require.Contains(t, joined, "insertRedactedNote(")

	listener, err := os.ReadFile(filepath.Join(root, "app/src/main/java/com/chenbenja/daywire/DaywireNotificationListener.kt"))
	require.NoError(t, err)
	require.Contains(t, string(listener), "insertNotification(")
	require.NotContains(t, string(listener), "insertRedactedNote(")
	require.NotContains(t, string(listener), "insertCoarseVisit(")

	manifest, err := os.ReadFile(filepath.Join(root, "app/src/main/AndroidManifest.xml"))
	require.NoError(t, err)
	manifestText := string(manifest)
	require.Contains(t, manifestText, "NotificationListenerService")
	require.Contains(t, manifestText, "ACCESS_COARSE_LOCATION")
	require.Contains(t, manifestText, "POST_NOTIFICATIONS")
	require.Contains(t, manifestText, "allowBackup=\"false\"")
}
