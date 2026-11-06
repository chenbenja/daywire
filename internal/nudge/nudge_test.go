package nudge_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/nudge"
	"github.com/chenbenja/daywire/internal/testutil"
)

func TestLogFallbackIncludesNudgeText(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	sender := nudge.New("", "", logger)
	res, err := sender.Deliver(testutil.Context(t), nudge.Proposal{
		Title:   "Synthetic nudge",
		Message: "Pack the spare charger.",
	})
	require.NoError(t, err)
	require.True(t, res.Delivered)
	require.Equal(t, "log", res.Channel)
	require.Contains(t, buf.String(), "Pack the spare charger.")
	require.Contains(t, buf.String(), "Synthetic nudge")
}

func TestNtfyPost(t *testing.T) {
	var gotBody, gotTitle, gotAuth, gotPriority string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		gotBody = string(body)
		gotTitle = r.Header.Get("Title")
		gotAuth = r.Header.Get("Authorization")
		gotPriority = r.Header.Get("Priority")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	sender := nudge.New(srv.URL+"/secret-topic", "token-value", testutil.Logger(t))
	res, err := sender.Deliver(testutil.Context(t), nudge.Proposal{
		Title:    "Synthetic nudge",
		Message:  "Pack the spare charger.",
		Priority: 3,
	})
	require.NoError(t, err)
	require.True(t, res.Delivered)
	require.Equal(t, "ntfy", res.Channel)
	require.Equal(t, "posted", res.Detail)
	require.Equal(t, "Pack the spare charger.", gotBody)
	require.Equal(t, "Synthetic nudge", gotTitle)
	require.Equal(t, "Bearer token-value", gotAuth)
	require.Equal(t, "3", gotPriority)
}

func TestDeliveryErrorOmitsBodyAndTopic(t *testing.T) {
	const (
		message = "UNIQUE-NUDGE-BODY-zzz"
		topic   = "secret-topic-path"
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, message, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	sender := nudge.New(srv.URL+"/"+topic, "", testutil.Logger(t))
	res, err := sender.Deliver(testutil.Context(t), nudge.Proposal{Title: "Title", Message: message})
	require.Error(t, err)
	require.False(t, res.Delivered)
	require.NotContains(t, err.Error(), message)
	require.NotContains(t, err.Error(), topic)
	require.NotContains(t, res.Detail, message)
	require.Contains(t, res.Detail, "500")
}

func TestValidate(t *testing.T) {
	t.Parallel()
	err := (nudge.Proposal{Title: "bad\ntitle", Message: "ok"}).Validate()
	require.ErrorContains(t, err, "nudge title is invalid")
	require.NotContains(t, err.Error(), "bad")
	err = (nudge.Proposal{Title: "ok", Message: "", Priority: 9}).Validate()
	require.Error(t, err)
}
