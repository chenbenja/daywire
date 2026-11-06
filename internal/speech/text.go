package speech

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/chenbenja/daywire/internal/event"
	"github.com/chenbenja/daywire/internal/voiceprint"
)

const (
	// LangEnglish is the default Moonshine tiny English path.
	LangEnglish = "en"
	// LangHebrew is the ivrit-ai Zipformer path. It is selected by the user.
	LangHebrew = "he"
	// SourceID is the producer stored with every speech turn. It is not a
	// file name and not a path.
	SourceID = "android:speech"
)

// Summary returns the note text for one speaker turn.
//
// language is en or he. label is owner, other, or unknown. text is the
// transcript. Control characters are stripped, the result is capped to the
// store summary limit, and the error text never quotes text. The summary
// does not include a file path because this function does not take one.
func Summary(language, label, text string) (string, error) {
	switch language {
	case LangEnglish, LangHebrew:
	default:
		return "", fmt.Errorf("invalid speech language")
	}
	switch label {
	case voiceprint.OwnerLabel, voiceprint.OtherLabel, voiceprint.UnknownLabel:
	default:
		return "", fmt.Errorf("invalid speech label")
	}
	clean := stripControls(text)
	if clean == "" {
		return "", fmt.Errorf("speech text is empty")
	}
	prefix := language + " " + label + ": "
	if len(prefix) >= event.MaxSummaryBytes {
		return "", fmt.Errorf("speech text is invalid")
	}
	summary := prefix + capBytes(clean, event.MaxSummaryBytes-len(prefix))
	if !utf8.ValidString(summary) || len(summary) > event.MaxSummaryBytes {
		return "", fmt.Errorf("speech text is invalid")
	}
	return summary, nil
}

func stripControls(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func capBytes(text string, budget int) string {
	if budget <= 0 || text == "" {
		return ""
	}
	if len(text) <= budget {
		return text
	}
	cut := text[:budget]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return strings.TrimSpace(cut)
}
