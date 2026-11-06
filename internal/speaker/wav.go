package speaker

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

// SampleRate is the rate the laptop models accept.
const SampleRate = 16000

// maxWAVBytes is a cap so a mistaken path cannot load an arbitrary file.
// A two-minute 16 kHz mono PCM clip is about 4 MB.
const maxWAVBytes = 8 << 20

// ReadWAV reads a 16-bit PCM WAV. The file must be mono. Sample values are
// scaled by 1/32768, which matches sherpa-onnx.
func ReadWAV(ctx context.Context, path string) (sampleRate int, samples []float32, err error) {
	if err := ctx.Err(); err != nil {
		return 0, nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, nil, fmt.Errorf("open wav: %w", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, nil, fmt.Errorf("stat wav: %w", err)
	}
	if st.Size() < 44 || st.Size() > maxWAVBytes {
		return 0, nil, fmt.Errorf("wav size %d is outside the spike limit", st.Size())
	}
	return decodeWAV(f)
}

func decodeWAV(r io.Reader) (sampleRate int, samples []float32, err error) {
	header := make([]byte, 12)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, fmt.Errorf("read wav header: %w", err)
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return 0, nil, fmt.Errorf("wav is not RIFF/WAVE")
	}
	var (
		format   uint16
		channels uint16
		rate     uint32
		bits     uint16
		data     []byte
		sawFmt   bool
	)
	for {
		var chunk [8]byte
		if _, err := io.ReadFull(r, chunk[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return 0, nil, fmt.Errorf("read wav chunk: %w", err)
		}
		id := string(chunk[0:4])
		size := binary.LittleEndian.Uint32(chunk[4:8])
		if size > maxWAVBytes {
			return 0, nil, fmt.Errorf("wav chunk is too large")
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(r, payload); err != nil {
			return 0, nil, fmt.Errorf("read wav %s chunk: %w", id, err)
		}
		if size%2 == 1 {
			if _, err := io.CopyN(io.Discard, r, 1); err != nil && err != io.EOF {
				return 0, nil, fmt.Errorf("skip wav pad: %w", err)
			}
		}
		switch id {
		case "fmt ":
			if len(payload) < 16 {
				return 0, nil, fmt.Errorf("wav fmt chunk is short")
			}
			format = binary.LittleEndian.Uint16(payload[0:2])
			channels = binary.LittleEndian.Uint16(payload[2:4])
			rate = binary.LittleEndian.Uint32(payload[4:8])
			bits = binary.LittleEndian.Uint16(payload[14:16])
			sawFmt = true
		case "data":
			data = payload
		}
	}
	if !sawFmt || data == nil {
		return 0, nil, fmt.Errorf("wav is missing fmt or data")
	}
	if format != 1 {
		return 0, nil, fmt.Errorf("wav format %d is not 16-bit PCM", format)
	}
	if channels != 1 {
		return 0, nil, fmt.Errorf("wav has %d channels, want mono", channels)
	}
	if bits != 16 {
		return 0, nil, fmt.Errorf("wav is %d-bit, want 16-bit", bits)
	}
	if rate == 0 || rate > 96000 {
		return 0, nil, fmt.Errorf("wav sample rate %d is not usable", rate)
	}
	if len(data) < 2 || len(data)%2 != 0 {
		return 0, nil, fmt.Errorf("wav data length is not 16-bit samples")
	}
	return int(rate), pcm16ToFloat(data), nil
}

func pcm16ToFloat(data []byte) []float32 {
	out := make([]float32, len(data)/2)
	for i := range out {
		v := int16(binary.LittleEndian.Uint16(data[i*2:]))
		out[i] = float32(v) / 32768.0
	}
	return out
}

// WriteWAV writes mono 16-bit PCM. Values outside [-1, 1] are clipped.
func WriteWAV(ctx context.Context, path string, sampleRate int, samples []float32) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if sampleRate <= 0 {
		return fmt.Errorf("invalid sample rate")
	}
	if len(samples) == 0 {
		return fmt.Errorf("wav has no samples")
	}
	data := floatToPCM16(samples)
	buf := make([]byte, 44+len(data))
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+len(data)))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], 1)
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(buf[32:34], 2)
	binary.LittleEndian.PutUint16(buf[34:36], 16)
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(len(data)))
	copy(buf[44:], data)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("write wav: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(buf); err != nil {
		return fmt.Errorf("write wav: %w", err)
	}
	return nil
}

func floatToPCM16(samples []float32) []byte {
	out := make([]byte, len(samples)*2)
	for i, s := range samples {
		if s > 1 {
			s = 1
		} else if s < -1 {
			s = -1
		}
		v := int16(math.Round(float64(s) * 32767))
		binary.LittleEndian.PutUint16(out[i*2:], uint16(v))
	}
	return out
}
