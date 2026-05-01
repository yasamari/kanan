package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	gotmdb "github.com/cyruzin/golang-tmdb"
	"github.com/urfave/cli/v3"
	"github.com/yasamari/kanan/internal/processor"
	"github.com/yasamari/kanan/internal/record"
	"github.com/yasamari/kanan/internal/saya"
	"github.com/yasamari/kanan/internal/syoboi"
	"github.com/yasamari/kanan/internal/tmdb"
	"github.com/yasamari/kanan/internal/util"
	"golang.org/x/time/rate"
)

var tsExtensions = []string{".ts", ".m2ts", ".mts"}

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

			syoboiClient := syoboi.NewClient(http.Client{
				Transport: &util.RateLimitRoundTripper{
					Transport: http.DefaultTransport,
					Limiter:   rate.NewLimiter(rate.Every(1*time.Second), 1),
				},
			})

			tmdbClient, err := gotmdb.Init(cmd.String("apikey"))
			if err != nil {
				return fmt.Errorf("failed to initialize TMDB client: %w", err)
			}
			tmdbClient.SetClientAutoRetry()

			serviceToChannelID, err := saya.GetServiceToChannelIDMap()
			if err != nil {
				return fmt.Errorf("failed to get service to channel ID map: %w", err)
			}

			infoExtractor := record.NewTsInfoExtractor()
			episodeResolver := tmdb.NewEpisodeResolver(tmdbClient)
			syoboiResolver := syoboi.NewResolver(syoboiClient, serviceToChannelID)

			proc := processor.New(infoExtractor, syoboiResolver, episodeResolver)

			isDir := false

			if info, err := os.Stat(path); os.IsNotExist(err) {
				return fmt.Errorf("file does not exist: %s", path)
			} else {
				isDir = info.IsDir()
			}

			if !isDir {
				result, err := proc.ProcessMultiple([]string{path}, rootDir, cmd.Bool("dryrun"))
				if err != nil {
					return fmt.Errorf("failed to process file: %w", err)
				}
				if err := createSymlinks(result, rootDir, cmd.Bool("dryrun")); err != nil {
					return fmt.Errorf("failed to create symlink: %w", err)
				}
				return nil
			}

			entries, err := os.ReadDir(path)
			if err != nil {
				return fmt.Errorf("failed to read directory: %w", err)
			}

			paths := make([]string, 0, len(entries))
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				if !slices.Contains(tsExtensions, filepath.Ext(entry.Name())) {
					continue
				}

				entryPath := filepath.Join(path, entry.Name())
				paths = append(paths, entryPath)
			}

			result, err := proc.ProcessMultiple(paths, rootDir, cmd.Bool("dryrun"))
			if err != nil {
				return fmt.Errorf("failed to process files: %w", err)
			}
			if err := createSymlinks(result, rootDir, cmd.Bool("dryrun")); err != nil {
				return fmt.Errorf("failed to create symlinks: %w", err)
			}

			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

const (
	seriesDirFormat   = "%s (%s) [tmdbid-%d]"
	seasonDirFormat   = "Season %02d [syobocalid-%d]"
	episodeFileFormat = "%s S%02dE%02d [syobocalid-%d]%s"
)

func createSymlinks(processResults []processor.ProcessResult, rootDir string, dryRun bool) error {
	for _, result := range processResults {
		resolveResult := result.EpisodeResolveResult

		seriesDirName := fmt.Sprintf(seriesDirFormat, resolveResult.ShowName, resolveResult.FirstAirDate.Format("2006"), resolveResult.ShowID)
		seasonDirName := fmt.Sprintf(seasonDirFormat, resolveResult.SeasonNumber, result.SyoboiTitleID)
		episodeFileName := fmt.Sprintf(episodeFileFormat, resolveResult.EpisodeName, resolveResult.SeasonNumber, resolveResult.EpisodeNumber, result.SyoboiProgramID, filepath.Ext(result.Path))

		targetDir := filepath.Join(rootDir, seriesDirName, seasonDirName)
		targetPath := filepath.Join(targetDir, episodeFileName)

		if dryRun {
			fmt.Printf("Would create symlink: %s -> %s\n", targetPath, result.Path)
			continue
		}

		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
		}

		if _, err := os.Stat(targetPath); err == nil {
			fmt.Printf("Symlink already exists: %s\n", targetPath)
			continue
		}
		absPath, err := filepath.Abs(result.Path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %s: %w", result.Path, err)
		}
		if err := os.Symlink(absPath, targetPath); err != nil {
			return fmt.Errorf("failed to create symlink for %s: %w", result.Path, err)
		}
	}
	return nil
}
