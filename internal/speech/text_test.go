package speech

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/event"
)

func TestSummaryStripsAndCaps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		language string
		label    string
		text     string
		want     string
		wantErr  string
	}{
		{
			name: "english owner", language: "en", label: "owner", text: "hello there",
			want: "en owner: hello there",
		},
		{
			name: "hebrew other", language: "he", label: "other", text: "שלום",
			want: "he other: שלום",
		},
		{
			name: "unknown", language: "en", label: "unknown", text: "maybe",
			want: "en unknown: maybe",
		},
		{
			name: "controls stripped", language: "en", label: "owner", text: "hello\nthere\x00",
			want: "en owner: hellothere",
		},
		{
			name: "format chars stripped", language: "en", label: "owner", text: "ab\u200ec",
			want: "en owner: abc",
		},
		{
			name: "blank", language: "en", label: "owner", text: " \n\t",
			wantErr: "speech text is empty",
		},
		{
			name: "bad language", language: "fr", label: "owner", text: "secret phrase",
			wantErr: "invalid speech language",
		},
		{
			name: "bad label", language: "en", label: "Owner", text: "secret phrase",
			wantErr: "invalid speech label",
		},
		{
			name: "path is not a parameter", language: "en", label: "other", text: "said a slash",
			want: "en other: said a slash",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Summary(tc.language, tc.label, tc.text)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.EqualError(t, err, tc.wantErr)
				require.NotContains(t, err.Error(), "secret")
				require.NotContains(t, err.Error(), "hello")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			require.NotContains(t, got, "\n")
			require.NotContains(t, got, "/")
		})
	}
}

func TestSummaryCapsOnRuneBoundary(t *testing.T) {
	t.Parallel()
	// Hebrew letters are 2 bytes in UTF-8. A long turn must stay valid and
	// inside the store limit, including the "en owner: " prefix.
	text := strings.Repeat("א", 2000)
	got, err := Summary("en", "owner", text)
	require.NoError(t, err)
	require.LessOrEqual(t, len(got), event.MaxSummaryBytes)
	require.Greater(t, len(got), len("en owner: "))
	require.Contains(t, got, "en owner: ")
	require.NotContains(t, got, strings.Repeat("א", 2000))
}

func TestSummaryDoesNotTakeAPath(t *testing.T) {
	t.Parallel()
	got, err := Summary("en", "owner", "pack the charger")
	require.NoError(t, err)
	require.Equal(t, SourceID, "android:speech")
	require.NotContains(t, got, "wav")
	require.NotContains(t, SourceID, "/")
}
