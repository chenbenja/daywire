package proposal

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxTitleRunes   = 120
	maxMessageRunes = 512
)

// Proposal is a redacted nudge. It is not a tool call and it never contains
// call audio.
type Proposal struct {
	Title    string
	Message  string
	Priority int
}

// Validate checks the proposal. Errors do not quote the title or message.
func (p Proposal) Validate() error {
	if strings.TrimSpace(p.Title) == "" {
		return fmt.Errorf("nudge title is empty")
	}
	if !validHeaderValue(p.Title) || utf8.RuneCountInString(p.Title) > maxTitleRunes {
		return fmt.Errorf("nudge title is invalid")
	}
	if strings.TrimSpace(p.Message) == "" {
		return fmt.Errorf("nudge message is empty")
	}
	if strings.ContainsRune(p.Message, 0) || !utf8.ValidString(p.Message) || utf8.RuneCountInString(p.Message) > maxMessageRunes {
		return fmt.Errorf("nudge message is invalid")
	}
	if p.Priority < 0 || p.Priority > 5 {
		return fmt.Errorf("nudge priority must be 0..5")
	}
	return nil
}

func validHeaderValue(s string) bool {
	if strings.TrimSpace(s) == "" || strings.ContainsAny(s, "\r\n") {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return utf8.ValidString(s)
}
