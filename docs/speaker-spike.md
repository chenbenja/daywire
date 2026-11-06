# Laptop speaker-id spike

This is a laptop check that enrollment can be matched to a speaker cluster. The phone path is [Week 3](poc-week3-speech.md): it stores the same encrypted embedding and scores each window `owner`, `other`, or `unknown` instead of clustering a whole file. This command does not add an audio field, an upload route, or a network bind. `serve` and `doctor` still do not link sherpa.

## What it proves

1. Embed a short public WAV and store that vector under the data directory, encrypted with the same AES-256-GCM key as the event store.
2. Diarize a two-speaker clip on CPU and label the cluster that matches the enrollment as `owner`.

The engine is [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx) v2.0.1 through its Go bindings (`github.com/k2-fsa/sherpa-onnx-go/v2`). Those bindings link the project's prebuilt `libsherpa-onnx-c-api` and `libonnxruntime`. The embedding model is NVIDIA NeMo `speakerverification_speakernet` (256 dimensions). Segmentation is pyannote segmentation 3.0. On the LibriSpeech fixtures below, the enrolled reader scores about 0.73 and the other reader about 0.24.

## Two binaries

`./hack/make.sh all` builds `output/daywire` with `CGO_ENABLED=0`. That is the Week 1 server. It does not link sherpa or onnxruntime, so `serve` and `doctor` do not depend on the module cache. `daywire speaker` in that binary exits with `speaker engine unavailable`.

The spike is opt-in:

```bash
./hack/make.sh speaker
./output/daywire-speaker speaker demo --data-dir /tmp/daywire-speaker
```

`hack/make.sh speaker` runs `go test -tags speaker` and writes `output/daywire-speaker`. That binary links the native libraries with an rpath into the Go module cache, so run it on the machine that built it. Do not use it as the long-running MCP server. After `go clean -modcache` it will not start until it is rebuilt.

`speaker enroll` and `speaker label` on that binary take a 16 kHz mono PCM wav. `--speakers 2` forces two clusters. The default `0` uses sherpa's cosine-distance threshold (0.5). `--id` defaults to `owner`.

The first spike run downloads two models into `output/speaker-models/` when the working directory is this module (`github.com/chenbenja/daywire`). Another checkout is left alone; the cache is then `$XDG_CACHE_HOME/daywire/speaker` or `~/.cache/daywire/speaker`. `DAYWIRE_SPEAKER_MODEL_DIR` overrides both. The files are pinned by SHA-256 in `internal/speaker/models.go` and are not committed. If the cache is empty and the download fails, the command exits with `speaker models unavailable`.

## What stays local

- The embedding file is `<data-dir>/speakers/<id>.emb`, mode `0600`, directory mode `0700`. The vector is ciphertext. The authenticated data is the speaker id and the column `speaker.embedding`, which is not an event or nudge column.
- Diarization logs speaker ids, times, and cosine scores. It does not log the vector or the samples.
- No event row is written. `events.db` is not opened.
- Model download is a GET of the two pinned sherpa-onnx release URLs. Nothing is posted.

## Fixtures

LibriSpeech test-clean, CC BY 4.0. See [internal/speaker/testdata/README.md](../internal/speaker/testdata/README.md).
