package command

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/urfave/cli/v3"
	"github.com/yasamari/kanan/internal/processor"
	"github.com/yasamari/kanan/internal/record"
	"github.com/yasamari/kanan/internal/syoboi"
)

func NewFileCommand() *cli.Command {
	return &cli.Command{
		Name:  "file",
		Usage: "Process a single recorded TV file",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "filepath",
				UsageText: "Path to the recorded TV file",
				Value:     "",
			},
		},

		Action: func(ctx context.Context, cmd *cli.Command) error {
			logLevel := slog.LevelInfo
			if cmd.Bool("verbose") {
				logLevel = slog.LevelDebug
			}

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
			slog.SetDefault(logger)

			if cmd.StringArg("filepath") == "" {
				return fmt.Errorf("filepath argument is required")
			}

			syoboiClient := syoboi.NewClient()
			tmdbClient, err := tmdb.Init(cmd.String("apikey"))
			if err != nil {
				return fmt.Errorf("failed to initialize TMDB client: %w", err)
			}
			infoExtractor := record.NewTsInfoExtractor()

			processor := processor.New(syoboiClient, tmdbClient, infoExtractor)

			if err := processor.Process(cmd.StringArg("filepath"), cmd.String("rootdir"), cmd.Bool("dryrun")); err != nil {
				return fmt.Errorf("failed to process file: %w", err)
			}

			return nil
		},
	}
}
