// Package speaker is the laptop speaker-identification spike.
//
// It enrolls one voice from a short WAV, stores that embedding on disk, and
// later labels the matching diarized cluster as "owner". This is a local
// proof for the later on-device path. It is not speech-to-text, not an audio
// upload, and not a field on semantic events. Assistants still see only
// redacted text. Embeddings stay on the machine; they are AES-256-GCM
// ciphertext and are not sent to a model API.
package speaker
