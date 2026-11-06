package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/urfave/cli/v3"

	"github.com/chenbenja/daywire/internal/speaker"
)

func speakerCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "speaker",
		Usage: "laptop speaker-id spike (local enroll and owner label; not an assistant audio path)",
		Commands: []*cli.Command{
			{
				Name:  "enroll",
				Usage: "embed one 16 kHz mono wav and store it for the owner",
				Flags: speakerFileFlags(true),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return speakerEnroll(ctx, cmd, logger)
				},
			},
			{
				Name:  "label",
				Usage: "diarize a wav and label the cluster that matches the enrollment",
				Flags: append(speakerFileFlags(true), speakersFlag()),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return speakerLabel(ctx, cmd, logger)
				},
			},
			{
				Name:  "demo",
				Usage: "enroll a LibriSpeech clip and label that reader in a two-speaker mix",
				Flags: append(speakerFileFlags(false), speakersFlag(), &cli.StringFlag{
					Name:  "fixture-dir",
					Usage: "directory of the committed public wavs (default internal/speaker/testdata)",
				}),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return speakerDemo(ctx, cmd, logger)
				},
			},
		},
	}
}

func speakerFileFlags(wav bool) []cli.Flag {
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:    "data-dir",
			Usage:   "data directory for the encrypted embedding (env DAYWIRE_DATA_DIR)",
			Sources: cli.EnvVars("DAYWIRE_DATA_DIR"),
		},
		&cli.StringFlag{
			Name:    "model-dir",
			Usage:   "checksummed model cache (env DAYWIRE_SPEAKER_MODEL_DIR)",
			Sources: cli.EnvVars("DAYWIRE_SPEAKER_MODEL_DIR"),
		},
		&cli.StringFlag{
			Name:  "id",
			Usage: "enrollment id",
			Value: "owner",
		},
	}
	if wav {
		flags = append(flags, &cli.StringFlag{
			Name:  "wav",
			Usage: "16 kHz mono 16-bit PCM wav",
		})
	}
	return flags
}

func speakersFlag() cli.Flag {
	return &cli.IntFlag{
		Name:  "speakers",
		Usage: "known speaker count; 0 clusters by cosine distance",
	}
}

func speakerEnroll(ctx context.Context, cmd *cli.Command, logger *slog.Logger) error {
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	id := cmd.String("id")
	rate, samples, err := readSpeakerWAV(ctx, cmd.String("wav"))
	if err != nil {
		return err
	}
	engine, err := openSpeakerEngine(ctx, cmd, logger)
	if err != nil {
		return err
	}
	defer engine.Close()
	dim, err := speaker.Enroll(ctx, engine, cfg.DataDir, cfg.DataKey, id, samples, rate)
	if err != nil {
		return err
	}
	logger.InfoContext(ctx, "enroll succeeded", "speaker", id, "dim", dim)
	return nil
}

func speakerLabel(ctx context.Context, cmd *cli.Command, logger *slog.Logger) error {
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	id := cmd.String("id")
	n, err := speakerCount(cmd)
	if err != nil {
		return err
	}
	rate, samples, err := readSpeakerWAV(ctx, cmd.String("wav"))
	if err != nil {
		return err
	}
	engine, err := openSpeakerEngine(ctx, cmd, logger)
	if err != nil {
		return err
	}
	defer engine.Close()
	res, err := speaker.LabelClip(ctx, engine, cfg.DataDir, cfg.DataKey, id, samples, rate, n)
	if err != nil {
		return err
	}
	logSpeakerResult(ctx, logger, res)
	return nil
}

func speakerDemo(ctx context.Context, cmd *cli.Command, logger *slog.Logger) error {
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	id := cmd.String("id")
	n, err := speakerCount(cmd)
	if err != nil {
		return err
	}
	dir, err := speaker.ResolveFixtureDir(cmd.String("fixture-dir"))
	if err != nil {
		return err
	}
	clip, err := speaker.LoadDemo(ctx, dir)
	if err != nil {
		return err
	}
	engine, err := openSpeakerEngine(ctx, cmd, logger)
	if err != nil {
		return err
	}
	defer engine.Close()
	dim, err := speaker.Enroll(ctx, engine, cfg.DataDir, cfg.DataKey, id, clip.Enroll, clip.SampleRate)
	if err != nil {
		return err
	}
	logger.InfoContext(ctx, "enroll succeeded", "speaker", id, "dim", dim)
	res, err := speaker.LabelClip(ctx, engine, cfg.DataDir, cfg.DataKey, id, clip.Mixed, clip.SampleRate, n)
	if err != nil {
		return err
	}
	logSpeakerResult(ctx, logger, res)
	return nil
}

func openSpeakerEngine(ctx context.Context, cmd *cli.Command, logger *slog.Logger) (speaker.Engine, error) {
	return speaker.NewEngine(ctx, cmd.String("model-dir"), logger)
}

func readSpeakerWAV(ctx context.Context, path string) (int, []float32, error) {
	if path == "" {
		return 0, nil, fmt.Errorf("wav is required")
	}
	rate, samples, err := speaker.ReadWAV(ctx, path)
	if err != nil {
		return 0, nil, err
	}
	if rate != speaker.SampleRate {
		return 0, nil, fmt.Errorf("wav sample rate is %d, want %d", rate, speaker.SampleRate)
	}
	return rate, samples, nil
}

func speakerCount(cmd *cli.Command) (int, error) {
	n := cmd.Int("speakers")
	if n < 0 {
		return 0, fmt.Errorf("speakers must be zero or more")
	}
	return n, nil
}

func logSpeakerResult(ctx context.Context, logger *slog.Logger, res speaker.Result) {
	for _, turn := range res.Turns {
		logger.InfoContext(ctx, "speaker turn",
			"speaker", turn.Speaker,
			"start", fmt.Sprintf("%.3f", turn.Start),
			"end", fmt.Sprintf("%.3f", turn.End),
			"label", turn.Label,
			"score", fmt.Sprintf("%.3f", turn.Score),
		)
	}
	logger.InfoContext(ctx, "owner labeled", "speaker", res.Owner)
}
