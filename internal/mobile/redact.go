package mobile

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/chenbenja/daywire/internal/event"
)

const (
	notificationSourcePrefix = "android:notification:"
	maxLabelRunes            = 80
	// maxPackageLen keeps "android:notification:" + package inside the source id limit.
	maxPackageLen = 107
)

// packageName matches a Java-style Android application id: at least two
// segments, each starting with a letter.
var packageName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z][A-Za-z0-9_]*)+$`)

// categoryWords maps Notification.category constants to words that may appear
// in a summary. Any other category string is dropped, so a notification body
// stuffed into the category field is not stored.
var categoryWords = map[string]string{
	"alarm":            "alarm",
	"call":             "call",
	"email":            "email",
	"err":              "error",
	"event":            "event",
	"msg":              "message",
	"navigation":       "navigation",
	"progress":         "progress",
	"promo":            "promo",
	"recommendation":   "recommendation",
	"reminder":         "reminder",
	"service":          "service",
	"social":           "social",
	"status":           "status",
	"sys":              "system",
	"transport":        "transport",
	"missed_call":      "missed call",
	"workout":          "workout",
	"location_sharing": "location sharing",
	"stopwatch":        "stopwatch",
}

// NotificationSummary builds the stored note for an allowed notification.
//
// appLabel is the PackageManager label, not the notification title. category
// is Notification.category. Unknown categories are omitted. The summary never
// includes a notification title or body because this function does not take them.
func NotificationSummary(appLabel, packageNameValue, category string) (string, error) {
	pkg, err := cleanPackage(packageNameValue)
	if err != nil {
		return "", err
	}
	label := cleanLabel(appLabel)
	if label == "" {
		label = pkg
	}
	summary := fmt.Sprintf("Notification: %s (%s)", label, pkg)
	if word := categoryWords[strings.TrimSpace(category)]; word != "" {
		summary += ", " + word
	}
	if len(summary) > event.MaxSummaryBytes || !utf8.ValidString(summary) {
		return "", fmt.Errorf("notification summary is invalid")
	}
	return summary, nil
}

// CoarseVisitSummary quantizes latitude and longitude to a 0.01 degree grid
// (about 1.1 km) and returns the visit summary. The raw values are not in the
// result. Callers must not log the inputs.
func CoarseVisitSummary(latitude, longitude float64) (string, error) {
	if math.IsNaN(latitude) || math.IsNaN(longitude) || math.IsInf(latitude, 0) || math.IsInf(longitude, 0) {
		return "", fmt.Errorf("invalid coordinates")
	}
	if latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return "", fmt.Errorf("invalid coordinates")
	}
	qlat := quantize(latitude)
	qlon := quantize(longitude)
	if qlat > 90 {
		qlat = 90
	}
	if qlat < -90 {
		qlat = -90
	}
	if qlon > 180 {
		qlon = 180
	}
	if qlon < -180 {
		qlon = -180
	}
	return fmt.Sprintf("Visit: coarse grid %.2f,%.2f", qlat, qlon), nil
}

func notificationSource(packageNameValue string) (string, error) {
	pkg, err := cleanPackage(packageNameValue)
	if err != nil {
		return "", err
	}
	return notificationSourcePrefix + pkg, nil
}

func cleanPackage(pkg string) (string, error) {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" || len(pkg) > maxPackageLen || !packageName.MatchString(pkg) {
		return "", fmt.Errorf("invalid package name")
	}
	return pkg, nil
}

func cleanLabel(label string) string {
	var b strings.Builder
	n := 0
	for _, r := range strings.TrimSpace(label) {
		if r < 0x20 || r == 0x7f {
			continue
		}
		if n == maxLabelRunes {
			break
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}

func quantize(v float64) float64 {
	q := math.Round(v*100) / 100
	if q == 0 {
		return 0
	}
	return q
}
