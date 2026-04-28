package main

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

func main() {
	cmd := &cli.Command{
		Name:  "kanan",
		Usage: "Organize recorded TV files based on Syoboi calendar and TMDB information",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "filepath",
				UsageText: "Path to the recorded TV file",
				Value:     "",
			},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "rootdir",
				Usage:   "Path to the root directory after organization",
				Aliases: []string{"r"},
				Value:   "",
			},
			&cli.BoolFlag{
				Name:    "dryrun",
				Usage:   "Display processing without executing file operations",
				Aliases: []string{"d"},
				Value:   false,
			},
			&cli.StringFlag{
				Name:    "apikey",
				Usage:   "TMDB API key (can also be set via TMDB_API_KEY environment variable)",
				Sources: cli.EnvVars("TMDB_API_KEY"),
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Display detailed output for debugging",
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

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
