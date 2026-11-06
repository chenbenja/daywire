# Daywire Week 3 — on-device phone speech

**Status:** the Android app can enroll an owner voice, forget it, and turn one picked WAV into speaker-labeled text events. November 2026.

Speech runs on the phone. Raw audio, embeddings, and transcripts are not uploaded. Assistants still receive only redacted events, and only later through the existing hub. This slice does not add an MCP audio field, an attachment route, or a network bind.

This is not call recording, not a microphone, and not an on-device LLM. It does not claim the Hebrew product gate is passed: there are no phone RAM or real-time-factor numbers, and no accuracy run on the kind of audio Daywire will eventually see.

## What shipped

| Piece | Where | Behavior |
|---|---|---|
| Voiceprint file | `internal/voiceprint` | Same AES-256-GCM file as the laptop spike: `<data-dir>/speakers/owner.emb`, directory `0700`, file `0600`, column `speaker.embedding`. |
| Turn label | `voiceprint.LabelTurn` | Cosine against that file. At or above 0.45 is `owner`. At or below 0.30 is `other`. The open interval, and a non-finite score, is `unknown`. |
| Speech note | `internal/speech`, `internal/mobile` | One `note` per turn. Source id `android:speech`. Summary `en owner: …`, `he other: …`, or `en unknown: …`. Control and format characters stripped, summary capped at 1024 bytes. No file path parameter. |
| Pipeline | `android/app/src/speech/` | 10 s windows, pyannote segmentation 3.0, NeMo SpeakerNet embed, cosine in Go, then STT. Compiled only when the sherpa-onnx AAR is present. |
| English STT | Moonshine tiny | About 5 s slices, MIT English model. |
| Hebrew STT | ivrit-ai Zipformer2 | User-selected. About 1.28 s streaming chunks, then 1.5 s of silence so the last chunk flushes. |
| UI | main screen | Enroll, forget, process, English or Hebrew, model and enrollment status. Work is off the main thread. Status survives rotation. New notes show in Recent events. |

`./hack/make.sh all` stays `gofmt`, `go vet`, `go test -race`, and `CGO_ENABLED=0` for `output/daywire`. It does not download models and it does not build the APK. `./hack/make.sh speech-models` is the pinned fetch. `./hack/android-bind.sh` still does not pass `-tags speaker` and still rejects sherpa or onnxruntime inside `daywire.aar`.

The laptop `daywire-speaker` binary is unchanged in role. It still clusters a clip. The phone does not: each window is scored against `owner.emb`, so a long file is not one clustering pass.

## Models and licences

Runtime: [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx) **v1.12.28** (Apache-2.0). The Android archive is `sherpa-onnx-1.12.28.aar` (about 39 MB). It bundles onnxruntime (MIT) for `arm64-v8a`, `armeabi-v7a`, `x86`, and `x86_64`. The app packages only `arm64-v8a` and `x86_64`, matching the Go bind. SHA-256 `b948066872e698f2e37e3c936c303426e12c5b7f9280e24f18767ed2efa08f8a`.

| File | Role | Licence | SHA-256 |
|---|---|---|---|
| `nemo_en_speakerverification_speakernet.onnx` | 256-d owner embedding | NVIDIA NeMo toolkit, Apache-2.0. NGC says the public SpeakerNet weights are covered by that toolkit licence. WeSpeaker was not required. | `d204dc8aac0014b8543f05fc8e310510c7022bc65b6452c203ec205ef7a66b23` |
| `sherpa-onnx-pyannote-segmentation-3-0/model.onnx` | who spoke when, inside each 10 s window | pyannote.audio segmentation 3.0, MIT | archive `24615ee884c897d9d2ba09bb4d30da6bb1b15e685065962db5b02e76e4996488`, model `220ad67ca923bef2fa91f2390c786097bf305bceb5e261d4af67b38e938e1079` |
| `sherpa-onnx-moonshine-tiny-en-int8.tar.bz2` | English STT | Moonshine tiny English, MIT (Useful Sensors). The archive's `LICENSE` file was checked. | `d5fe6ec4334fef36255b2a4010412cad4c007e33103fec62fb5d17cad88086f2` |
| `preprocess.onnx` | Moonshine | same | `f33addce61a143460fe753b5ee5b7db255e5140b5b779c065b94f6c83ff0bf4e` |
| `encode.int8.onnx` | Moonshine, about 18 MB | same | `8774dfba578de027ec6595c2c654a0836434489bc963a0db124a7f181f571acb` |
| `uncached_decode.int8.onnx` | Moonshine, about 51 MB | same | `216737000dd5881a17aa043f6bbd286add33e4c3b0ae257153e2ec15438bdc41` |
| `cached_decode.int8.onnx` | Moonshine, about 44 MB | same | `2aff28bba6a03d8dcf5c9feac45462629bae37317442299f28115ad09da773f6` |
| `tokens.txt` | Moonshine | same | `1165c2aeb9f72f457a83be2d459a09054f27490acd9b41bd43794dfd25e296ea` |
| `encoder.int8.onnx` | Hebrew Zipformer2, about 68 MB | [ivrit-ai/zipformer2-66m-streaming-he](https://huggingface.co/ivrit-ai/zipformer2-66m-streaming-he), CC-BY-4.0 | `6c6febcb74a09435315a1d872a6539f6e7fc2c333533f14c74edc0e85c49ffcd` |
| `decoder.int8.onnx` | Hebrew, about 1.1 MB | same | `feb03e7c8aa761593a9284a6fa1e91259be3702de75f630062256c1a59c5a906` |
| `joiner.int8.onnx` | Hebrew, about 754 KB | same | `1d2c6d8e6dabc5eb52de52e95718e9bdfbffbfe7ead3808fb5ffd74e83bd9cae` |
| `tokens.txt` | Hebrew | same | `e4ebb5e72a6aeef47437a55fa2308ebbedb7c51c557de7a89bf965c23ea9af4a` |

The published Moonshine tiny int8 tarball is about 103 MB compressed and about 120 MB unpacked. A 29.9 MB figure matches the LiteRT `.litertlm` pack on Hugging Face, which this slice does not use. LiteRT is a second runtime and is out of scope. The Moonshine test WAVs in that tarball are not copied into the app.

Hashes live in `internal/speech/pins.go` and `hack/fetch-speech-models.sh`. A test fails if they drift apart, and if the SpeakerNet and segmentation hashes drift from `internal/speaker/models.go`.

## Fetch at build time

From the repository root, on a machine that is allowed to download:

```bash
./hack/fetch-speech-models.sh
```

That writes:

- `android/app/libs/sherpa-onnx-1.12.28.aar` (gitignored)
- `android/app/speech-assets/speech/…` (gitignored except `.gitkeep`)

Gradle adds the AAR and the `src/speech` sources only when the AAR file is present. Assets are copied from `speech-assets/` when that directory has files. The phone does not download models. If the AAR or the English models are absent, the app still opens and says the speech runtime or the speech models are not packaged. If Hebrew files are absent, English still runs, and choosing Hebrew reports **Hebrew models not packaged.**

Also build the Go archive, as in Week 2:

```bash
export ANDROID_HOME="$HOME/Android/Sdk"
./hack/android-bind.sh
```

Then Android Studio → File → Open → `android/`. Re-sync Gradle after the fetch so the speech sources are compiled in.

## What the screen does

The Speech card on the main screen:

1. **Enroll owner from a file.** One Storage Access Framework document. The embedding is stored with the Go API. A later enroll replaces it. Only the first 30 seconds are embedded.
2. **Forget enrollment.** Deletes `owner.emb`. Tapping it again is success.
3. **Process a file.** Another document picker, not the Week 2 folder list. That list stays names only. Audio is read in 10 second windows, not as one hour-long pass. Processing stops after 10 minutes and says so.
4. **English or Hebrew.** The choice is explicit. Mixed speech is not auto-routed.

Each stored turn is a `note` with source `android:speech`. Recent events shows it. Logs record the event id, `en` or `he`, the label, and a turn count. They do not record the transcript, the embedding, the samples, or the picked path.

The picker offers audio MIME types. The decoder accepts **16 kHz, mono, 16-bit PCM WAV** only. Anything else fails with "That file is not 16 kHz mono PCM WAV." There is no resampler and no microphone permission.

## What is not in this slice

- Microphone, `RECORD_AUDIO`, an accessibility service, or a voice-call / call-recording capture
- `INTERNET` permission, or a model download from the phone
- An audio upload, an MCP audio field, or an attachment route
- Linking sherpa, onnxruntime, or `-tags speaker` into `daywire.aar` or into `serve` / `doctor`
- NVIDIA Sortformer, multitalker Parakeet, Parakeet TDT 0.6B, Whisper, Canary, SenseVoice, Qwen3-ASR, or LiteRT
- Moonshine models that are not English (Moonshine Community License)
- An on-device LLM, or invented commitments and asks from the transcript
- A Hebrew evaluation clip. No public Hebrew WAV with a licence checked in this change was added. Tests do not require the Hebrew files.
- Phone RAM, real-time factor, or accuracy numbers. The Hebrew kill gate is **not** claimed as passed.
- A `LICENSE` file. The README stays **License: TBD**.

## Tests

`go test` covers summary redaction and capping, owner / other / unknown on the documented LibriSpeech cosine bands (owner about 0.73, other about 0.24) and on the fixture timelines, voiceprint encryption and forget, and the mobile store round trip. The dependency check still rejects `net/http`, sherpa, onnxruntime, and `internal/speaker` inside the mobile bind. The Android source check still rejects `INTERNET`, `RECORD_AUDIO`, and opening files from the folder list. `openInputStream` is allowed only for the one picked speech file.

Kotlin unit tests in `SpeechGuardsTest` cover the same window cuts and the WAV decoder. They need the Android SDK, which this environment did not have, so they were not executed here. `./hack/android-bind.sh` and `./gradlew :app:testDebugUnitTest` were not run here for the same reason. The Go suite and `CGO_ENABLED=0` build were run.
