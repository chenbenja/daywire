// Package speaker is the laptop speaker-identification spike.
//
// It enrolls one voice from a short WAV, stores that embedding on disk, and
// later labels the matching diarized cluster as "owner". This is a local
// proof for the on-device path. The phone stores the same encrypted
// embedding and labels each window owner, other, or unknown. This package
// is not speech-to-text, not an audio upload, and not a field on semantic
// events. Assistants still see only redacted text. Embeddings stay on the
// machine; they are AES-256-GCM ciphertext and are not sent to a model API.
// The sherpa engine is behind the speaker build tag, so the default binary
// and the Android Go archive do not link it.
package speaker
