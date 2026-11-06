package speaker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// FixtureGapSeconds is the silence between the two speakers in the demo mix.
const FixtureGapSeconds = 0.5

const (
	// FixtureEnroll is the LibriSpeech utterance stored as the owner.
	FixtureEnroll = "librispeech-6930-75918-0000.wav"
	// FixtureOwner is a second utterance of that reader, held out of enrollment.
	FixtureOwner = "librispeech-6930-75918-0001.wav"
	// FixtureOther is a different LibriSpeech reader.
	FixtureOther = "librispeech-1320-122617-0000.wav"
)

const minEmbedSeconds = 0.5

// Turn is one diarized segment with an owner decision.
type Turn struct {
	Speaker int
	Start   float64
	End     float64
	Label   string
	Score   float64
}

// Result is the labeling of one clip. It is times and labels, not audio.
type Result struct {
	Turns []Turn
	Owner int
}

// DemoClip is the public-fixture enrollment and the mixed clip built from it.
type DemoClip struct {
	Enroll     []float32
	Mixed      []float32
	SampleRate int
	OwnerEnd   float64
	OtherStart float64
}

// LoadDemo reads the three LibriSpeech fixtures and builds the mixed clip:
// the second utterance of reader 6930, silence, then reader 1320.
// Enrollment is the first utterance of reader 6930, which is not copied into
// the mix.
func LoadDemo(ctx context.Context, fixtureDir string) (DemoClip, error) {
	enrollRate, enroll, err := ReadWAV(ctx, fixturePath(fixtureDir, FixtureEnroll))
	if err != nil {
		return DemoClip{}, err
	}
	ownerRate, owner, err := ReadWAV(ctx, fixturePath(fixtureDir, FixtureOwner))
	if err != nil {
		return DemoClip{}, err
	}
	otherRate, other, err := ReadWAV(ctx, fixturePath(fixtureDir, FixtureOther))
	if err != nil {
		return DemoClip{}, err
	}
	if enrollRate != SampleRate || ownerRate != SampleRate || otherRate != SampleRate {
		return DemoClip{}, fmt.Errorf("fixtures must be %d Hz", SampleRate)
	}
	gapN := int(FixtureGapSeconds * float64(SampleRate))
	mixed := make([]float32, 0, len(owner)+gapN+len(other))
	mixed = append(mixed, owner...)
	mixed = append(mixed, make([]float32, gapN)...)
	mixed = append(mixed, other...)
	ownerEnd := float64(len(owner)) / float64(SampleRate)
	return DemoClip{
		Enroll:     enroll,
		Mixed:      mixed,
		SampleRate: SampleRate,
		OwnerEnd:   ownerEnd,
		OtherStart: ownerEnd + FixtureGapSeconds,
	}, nil
}

func fixturePath(dir, name string) string {
	return filepath.Join(dir, name)
}

// ResolveFixtureDir finds the committed public clips.
// An empty dir means internal/speaker/testdata, from the working directory
// or from the module root when the command is run elsewhere in the repo.
func ResolveFixtureDir(dir string) (string, error) {
	if dir == "" {
		dir = filepath.Join("internal", "speaker", "testdata")
	}
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir, nil
	}
	root, err := moduleRoot()
	if err != nil {
		return "", fmt.Errorf("speaker fixtures not found")
	}
	candidate := dir
	if !filepath.IsAbs(dir) {
		candidate = filepath.Join(root, dir)
	}
	info, err := os.Stat(candidate)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("speaker fixtures not found")
	}
	return candidate, nil
}

// Enroll embeds samples and stores the vector for id.
func Enroll(ctx context.Context, engine Engine, dataDir string, key []byte, id string, samples []float32, sampleRate int) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	vec, err := engine.Embed(ctx, samples, sampleRate)
	if err != nil {
		return 0, err
	}
	if err := SaveProfile(ctx, dataDir, key, id, vec); err != nil {
		return 0, err
	}
	return len(vec), nil
}

// LabelClip diarizes samples and labels the cluster that matches id.
func LabelClip(ctx context.Context, engine Engine, dataDir string, key []byte, id string, samples []float32, sampleRate, speakers int) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	profile, err := LoadProfile(ctx, dataDir, key, id)
	if err != nil {
		return Result{}, err
	}
	segments, err := engine.Diarize(ctx, samples, sampleRate, speakers)
	if err != nil {
		return Result{}, err
	}
	if len(segments) == 0 {
		return Result{}, fmt.Errorf("diarization returned no speech")
	}
	grouped := map[int][][]float32{}
	order := make([]int, 0, 2)
	minSamples := int(minEmbedSeconds * float64(sampleRate))
	for _, seg := range segments {
		if seg.End <= seg.Start {
			return Result{}, fmt.Errorf("diarization returned an empty segment")
		}
		chunk := sliceSamples(samples, sampleRate, seg.Start, seg.End)
		if len(chunk) < minSamples {
			continue
		}
		vec, err := engine.Embed(ctx, chunk, sampleRate)
		if err != nil {
			return Result{}, err
		}
		if _, ok := grouped[seg.Speaker]; !ok {
			order = append(order, seg.Speaker)
		}
		grouped[seg.Speaker] = append(grouped[seg.Speaker], vec)
	}
	if len(grouped) == 0 {
		return Result{}, fmt.Errorf("diarization returned no segment long enough to embed")
	}
	scores := make([]ClusterScore, 0, len(order))
	scoreOf := map[int]float64{}
	for _, spk := range order {
		mean, err := MeanEmbedding(grouped[spk])
		if err != nil {
			return Result{}, err
		}
		score, err := Cosine(profile.Vec, mean)
		if err != nil {
			return Result{}, err
		}
		scores = append(scores, ClusterScore{Speaker: spk, Score: score})
		scoreOf[spk] = score
	}
	assigned, err := AssignOwner(scores, DefaultOwnerMinScore)
	if err != nil {
		return Result{}, err
	}
	turns := make([]Turn, 0, len(segments))
	for _, seg := range segments {
		label := assigned.Labels[seg.Speaker]
		if label == "" {
			label = OtherLabel
		}
		turns = append(turns, Turn{
			Speaker: seg.Speaker,
			Start:   seg.Start,
			End:     seg.End,
			Label:   label,
			Score:   scoreOf[seg.Speaker],
		})
	}
	sort.Slice(turns, func(i, j int) bool {
		if turns[i].Start == turns[j].Start {
			return turns[i].Speaker < turns[j].Speaker
		}
		return turns[i].Start < turns[j].Start
	})
	return Result{Turns: turns, Owner: assigned.Owner}, nil
}

func sliceSamples(samples []float32, sampleRate int, start, end float64) []float32 {
	a := int(start * float64(sampleRate))
	b := int(end * float64(sampleRate))
	if a < 0 {
		a = 0
	}
	if b > len(samples) {
		b = len(samples)
	}
	if b < a {
		return nil
	}
	return samples[a:b]
}
