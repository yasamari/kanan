package main

import (
	"context"
	"fmt"
	"log"
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
		Usage: "録画したtsファイルを整理するCLIツール",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "tspath",
				UsageText: "対象のファイルパス",
			},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "apikey",
				Aliases: []string{"a"},
				Usage:   "TMDB APIキー",
				Sources: cli.EnvVars("TMDB_API_KEY"),
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "詳細な出力を表示",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logLevel := slog.LevelInfo
			if cmd.Bool("verbose") {
				logLevel = slog.LevelDebug
			}

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
			slog.SetDefault(logger)

			syoboiClient := syoboi.NewClient()
			tmdbClient, err := tmdb.Init(cmd.String("apikey"))
			if err != nil {
				return fmt.Errorf("failed to initialize TMDB client: %w", err)
			}
			infoExtractor := record.NewTsInfoExtractor()

			err = processor.New(syoboiClient, tmdbClient, infoExtractor).Process(cmd.StringArg("tspath"))
			if err != nil {
				return err
			}

			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
