package speaker

import (
	"archive/tar"
	"compress/bzip2"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ErrModelsUnavailable means the pinned models are not cached and could not
// be downloaded. Callers that can run offline should skip with this error.
var ErrModelsUnavailable = errors.New("speaker models unavailable")

// Models holds local paths of the pinned ONNX files.
type Models struct {
	Embedding    string
	Segmentation string
}

const (
	// NeMo speakerverification_speakernet, English, 256-d.
	// Chosen over the larger Chinese ERes2Net and over CAM++ because, on the
	// public fixtures in testdata, same-speaker cosine is about 0.76 and
	// different-speaker cosine is about 0.18, and pyannote clustering splits
	// the mixed clip. SHA-256 is of the file published by sherpa-onnx v2.
	embeddingFile = "nemo_en_speakerverification_speakernet.onnx"
	embeddingURL  = "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-recongition-models/nemo_en_speakerverification_speakernet.onnx"
	embeddingSHA  = "d204dc8aac0014b8543f05fc8e310510c7022bc65b6452c203ec205ef7a66b23"
	embeddingMax  = 30 << 20

	segmentationTar    = "sherpa-onnx-pyannote-segmentation-3-0.tar.bz2"
	segmentationTarURL = "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-segmentation-models/sherpa-onnx-pyannote-segmentation-3-0.tar.bz2"
	segmentationTarSHA = "24615ee884c897d9d2ba09bb4d30da6bb1b15e685065962db5b02e76e4996488"
	segmentationTarMax = 12 << 20

	segmentationRel = "sherpa-onnx-pyannote-segmentation-3-0/model.onnx"
	segmentationSHA = "220ad67ca923bef2fa91f2390c786097bf305bceb5e261d4af67b38e938e1079"
	segmentationMax = 12 << 20
)

// EnsureModels returns the embedding and segmentation files, downloading
// them into dir when the checksummed cache is empty. Downloads are GET
// requests for these public files only.
func EnsureModels(ctx context.Context, dir string, logger *slog.Logger) (Models, error) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if err := ctx.Err(); err != nil {
		return Models{}, err
	}
	if dir == "" {
		return Models{}, fmt.Errorf("model dir is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Models{}, fmt.Errorf("create model cache: %w", err)
	}
	emb, err := ensureFile(ctx, dir, artifact{
		File: embeddingFile, URL: embeddingURL, SHA256: embeddingSHA, MaxBytes: embeddingMax,
	}, logger)
	if err != nil {
		return Models{}, err
	}
	tarPath, err := ensureFile(ctx, dir, artifact{
		File: segmentationTar, URL: segmentationTarURL, SHA256: segmentationTarSHA, MaxBytes: segmentationTarMax,
	}, logger)
	if err != nil {
		return Models{}, err
	}
	seg := filepath.Join(dir, segmentationRel)
	if ok, _ := fileSHA(seg, segmentationSHA); !ok {
		if err := extractSegmentation(tarPath, seg); err != nil {
			return Models{}, err
		}
		if ok, err := fileSHA(seg, segmentationSHA); !ok {
			if err != nil {
				return Models{}, err
			}
			return Models{}, fmt.Errorf("segmentation model checksum mismatch")
		}
	}
	logger.DebugContext(ctx, "speaker models ready")
	return Models{Embedding: emb, Segmentation: seg}, nil
}

type artifact struct {
	File     string
	URL      string
	SHA256   string
	MaxBytes int64
}

func ensureFile(ctx context.Context, dir string, a artifact, logger *slog.Logger) (string, error) {
	dest := filepath.Join(dir, a.File)
	if ok, _ := fileSHA(dest, a.SHA256); ok {
		return dest, nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	logger.InfoContext(ctx, "fetching speaker model", "file", a.File)
	tmp := fmt.Sprintf("%s.%d.partial", dest, os.Getpid())
	defer os.Remove(tmp)
	if err := download(ctx, a, tmp); err != nil {
		return "", err
	}
	if ok, err := fileSHA(tmp, a.SHA256); !ok {
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("speaker model %s checksum mismatch", a.File)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", fmt.Errorf("store speaker model: %w", err)
	}
	if ok, err := fileSHA(dest, a.SHA256); !ok {
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("speaker model %s checksum mismatch", a.File)
	}
	return dest, nil
}

func download(ctx context.Context, a artifact, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return fmt.Errorf("download %s: %w", a.File, err)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("download %s: %w", a.File, err)
		}
		return fmt.Errorf("%w: download %s: %s", ErrModelsUnavailable, a.File, err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", a.File, resp.StatusCode)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("store speaker model: %w", err)
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(resp.Body, a.MaxBytes+1))
	if err != nil {
		return fmt.Errorf("download %s: %w", a.File, err)
	}
	if n > a.MaxBytes {
		return fmt.Errorf("speaker model %s is larger than %d bytes", a.File, a.MaxBytes)
	}
	return nil
}

func fileSHA(path, want string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("hash speaker model: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, fmt.Errorf("hash speaker model: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	return got == want, nil
}

func extractSegmentation(tarPath, dest string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("open segmentation archive: %w", err)
	}
	defer f.Close()
	tr := tar.NewReader(bzip2.NewReader(f))
	var found bool
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read segmentation archive: %w", err)
		}
		if hdr.Name != segmentationRel || hdr.Typeflag != tar.TypeReg {
			continue
		}
		if err := writeLimited(dest, tr, segmentationMax); err != nil {
			return err
		}
		found = true
		break
	}
	if !found {
		return fmt.Errorf("segmentation archive has no model")
	}
	return nil
}

func writeLimited(dest string, r io.Reader, max int64) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create model cache: %w", err)
	}
	tmp := fmt.Sprintf("%s.%d.partial", dest, os.Getpid())
	defer os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("store segmentation model: %w", err)
	}
	n, copyErr := io.Copy(f, io.LimitReader(r, max+1))
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("store segmentation model: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("store segmentation model: %w", closeErr)
	}
	if n > max {
		return fmt.Errorf("segmentation model is larger than %d bytes", max)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("store segmentation model: %w", err)
	}
	return nil
}
