// Package speech builds speaker-labeled text for the on-device store.
//
// It does not run speech models, open a socket, or accept audio bytes.
// The phone passes a language, a speaker label, and transcript text. This
// package strips control characters, caps the summary, and refuses a file
// path. Window sizes match the Android pipeline: pyannote segmentation uses
// about 10 seconds, Moonshine about 5 seconds, and the Hebrew Zipformer
// about 1.28 seconds.
package speech
