package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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
				Name:      "path",
				UsageText: "Path to the recorded TV file",
				Value:     "",
			},
			&cli.StringArg{
				Name:      "rootdir",
				UsageText: "Path to the root directory after organization",
				Value:     "",
			},
		},
		Flags: []cli.Flag{
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

			path := cmd.StringArg("path")
			if path == "" {
				return fmt.Errorf("path argument is required")
			}
			rootDir := cmd.StringArg("rootdir")
			if rootDir == "" {
				return fmt.Errorf("rootdir argument is required")
			}

			syoboiClient := syoboi.NewClient()
			tmdbClient, err := tmdb.Init(cmd.String("apikey"))
			if err != nil {
				return fmt.Errorf("failed to initialize TMDB client: %w", err)
			}
			infoExtractor := record.NewTsInfoExtractor()

			processor := processor.New(syoboiClient, tmdbClient, infoExtractor)

			isDir := false

			if info, err := os.Stat(path); os.IsNotExist(err) {
				return fmt.Errorf("file does not exist: %s", path)
			} else {
				isDir = info.IsDir()
			}

			if !isDir {
				if err := processor.Process(path, rootDir, cmd.Bool("dryrun")); err != nil {
					return fmt.Errorf("failed to process file: %w", err)
				}
				return nil
			}

			entries, err := os.ReadDir(path)
			if err != nil {
				return fmt.Errorf("failed to read directory: %w", err)
			}

			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				entryPath := filepath.Join(path, entry.Name())
				if err := processor.Process(entryPath, rootDir, cmd.Bool("dryrun")); err != nil {
					slog.Error("Failed to process file", "path", entryPath, "error", err)
				}
			}

			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
