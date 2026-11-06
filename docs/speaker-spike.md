# Laptop speaker-id spike

This is a laptop check that enrollment can be matched to a speaker cluster. It is not the phone stack. On-device models (LiteRT or sherpa on Android) are Week 2–3. Week 1 assistants still receive only redacted text events. This command does not add an audio field, an upload route, or a network bind.

## What it proves

1. Embed a short public WAV and store that vector under the data directory, encrypted with the same AES-256-GCM key as the event store.
2. Diarize a two-speaker clip on CPU and label the cluster that matches the enrollment as `owner`.

The engine is [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx) v2.0.1 through its Go bindings (`github.com/k2-fsa/sherpa-onnx-go/v2`). Those bindings link the project's prebuilt `libsherpa-onnx-c-api` and `libonnxruntime` with an rpath into the module cache, so the spike does not compile sherpa or shell out to Python. A later phone build can keep the same enroll-then-match shape.

The embedding model is NVIDIA NeMo `speakerverification_speakernet` (256 dimensions, English). The segmentation model is pyannote segmentation 3.0. CAM++ was tried first: on these clips the same-speaker score was about 0.20 and a short-gap mix stayed one cluster. Speakernet scores the same speaker at about 0.76 and the other speaker at about 0.18, and the mix splits. The larger Chinese ERes2Net also worked; speakernet is the smaller English model.

## Run

From the repository root, after `./hack/make.sh all` (cgo on, which is the default when gcc is installed):

```bash
./output/daywire speaker demo --data-dir /tmp/daywire-speaker
```

That enrolls `speaker1_a_en_16k.wav` and labels a mix of `speaker1_b_en_16k.wav`, half a second of silence, and `speaker2_a_en_16k.wav`. The log lines `enroll succeeded` and `label=owner` are the result. The owner turns are the first speaker.

The same steps with your own 16 kHz mono PCM files:

```bash
./output/daywire speaker enroll --data-dir /tmp/daywire-speaker --wav enroll.wav
./output/daywire speaker label --data-dir /tmp/daywire-speaker --wav mixed.wav
```

`--speakers 2` forces two clusters. The default `0` uses sherpa's cosine-distance threshold (0.5). `--id` defaults to `owner`.

The first run downloads two models into `output/speaker-models/` (or `DAYWIRE_SPEAKER_MODEL_DIR`, or `~/.cache/daywire/speaker` when not in a checkout). The files are pinned by SHA-256 in `internal/speaker/models.go`. They are not committed. If the cache is empty and the download fails, the command exits with `speaker models unavailable`. `go test` skips that case the same way and fails if enrollment or the owner label is wrong once the models are present.

`output/daywire speaker` finds the shared libraries via the rpath recorded at `go build` time. Run the binary on the machine that built it.

## What stays local

- The embedding file is `<data-dir>/speakers/<id>.emb`, mode `0600`, directory mode `0700`. The vector is ciphertext. The id is not.
- Diarization logs speaker ids, times, and cosine scores. It does not log the vector or the samples.
- No event row is written. `events.db` is not opened.
- Model download is a GET of the two pinned sherpa-onnx release URLs. Nothing is posted.

## Fixtures

See [internal/speaker/testdata/README.md](../internal/speaker/testdata/README.md).
