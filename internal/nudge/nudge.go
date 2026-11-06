package nudge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chenbenja/daywire/internal/event"
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

// Result is the delivery status returned to the caller.
type Result struct {
	ID        string
	Channel   string
	Delivered bool
	Detail    string
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

// Sender posts nudges to a ntfy topic, or logs them when URL is empty.
//
// The log line includes the nudge text because that log is the delivery
// surface when ntfy is not configured. Other packages must not log nudge or
// event text.
type Sender struct {
	url    string
	token  string
	client *http.Client
	log    *slog.Logger
}

// New returns a sender. url is the full ntfy topic URL. An empty url selects
// the log channel. token, when set, is sent as a bearer token and is never logged.
func New(url, token string, logger *slog.Logger) *Sender {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Sender{
		url:   strings.TrimSpace(url),
		token: token,
		log:   logger,
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errors.New("ntfy redirect refused")
			},
		},
	}
}

// Deliver sends p. On a remote failure the result still carries an id and
// Delivered is false. The returned error does not include the message or the
// topic URL.
func (s *Sender) Deliver(ctx context.Context, p Proposal) (Result, error) {
	if err := p.Validate(); err != nil {
		return Result{}, err
	}
	id, err := event.NewID()
	if err != nil {
		return Result{}, err
	}
	if s.url == "" {
		s.log.InfoContext(ctx, "nudge logged", "nudge_id", id, "title", p.Title, "message", p.Message)
		return Result{ID: id, Channel: "log", Delivered: true, Detail: "logged"}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, strings.NewReader(p.Message))
	if err != nil {
		return Result{ID: id, Channel: "ntfy", Delivered: false, Detail: "invalid ntfy url"}, errors.New("invalid ntfy url")
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	req.Header.Set("Title", p.Title)
	if p.Priority > 0 {
		req.Header.Set("Priority", strconv.Itoa(p.Priority))
	}
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		detail := "post failed"
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			detail = "post timed out"
		}
		return Result{ID: id, Channel: "ntfy", Delivered: false, Detail: detail}, errors.New(detail)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := fmt.Sprintf("ntfy status %d", resp.StatusCode)
		return Result{ID: id, Channel: "ntfy", Delivered: false, Detail: detail}, errors.New(detail)
	}
	s.log.InfoContext(ctx, "nudge posted", "nudge_id", id, "channel", "ntfy")
	return Result{ID: id, Channel: "ntfy", Delivered: true, Detail: "posted"}, nil
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
