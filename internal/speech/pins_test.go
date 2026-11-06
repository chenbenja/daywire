package speech

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPinsMatchFetchScriptAndSpeakerModels(t *testing.T) {
	t.Parallel()
	script, err := os.ReadFile("../../hack/fetch-speech-models.sh")
	require.NoError(t, err)
	models, err := os.ReadFile("../speaker/models.go")
	require.NoError(t, err)
	kotlin, err := os.ReadFile("../../android/app/src/main/java/com/chenbenja/daywire/SpeechAssets.kt")
	require.NoError(t, err)

	for _, pin := range []string{
		SherpaAARURL, SherpaAARSHA, SherpaAARFile,
		EmbeddingURL, EmbeddingSHA,
		SegmentationTarURL, SegmentationTarSHA, SegmentationSHA,
		MoonshineTarURL, MoonshineTarSHA,
		MoonshinePreSHA, MoonshineEncSHA, MoonshineUncSHA, MoonshineDecSHA, MoonshineTokSHA,
		HebrewEncURL, HebrewDecURL, HebrewJoinURL, HebrewTokURL,
		HebrewEncSHA, HebrewDecSHA, HebrewJoinSHA, HebrewTokSHA,
	} {
		require.Contains(t, string(script), pin)
	}
	require.Contains(t, string(models), EmbeddingSHA)
	require.Contains(t, string(models), SegmentationSHA)
	require.Contains(t, string(models), SegmentationTarSHA)

	for _, asset := range append(EnglishAssets(), HebrewAssets()...) {
		require.Contains(t, string(kotlin), asset)
		require.True(t, strings.HasPrefix(asset, "speech/"))
		require.NotContains(t, asset, "..")
	}
}
