package processor

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/yasamari/kanan/internal/record"
	"github.com/yasamari/kanan/internal/syoboi"
	"github.com/yasamari/kanan/internal/tmdb"
)

type ProgressReporter interface {
	Start(phase string, total int)
	Update(current, total int, detail string)
	Message(message string)
	Done(message string)
}

var ErrNotFound = errors.New("not found")

type processor struct {
	infoExtractor   record.InfoExtractor
	episodeResolver tmdb.EpisodeResolver
	syoboiResolver  syoboi.Resolver
}

func New(infoExtractor record.InfoExtractor, syoboiResolver syoboi.Resolver, episodeResolver tmdb.EpisodeResolver) *processor {
	return &processor{
		syoboiResolver:  syoboiResolver,
		infoExtractor:   infoExtractor,
		episodeResolver: episodeResolver,
	}
}

const (
	createNewChunksThreshold = 24 * time.Hour * 30 // 1ヶ月以上離れているものは別のチャンクとして処理する
)

type pathChunk struct {
	Infos    []record.Info
	Earliest time.Time
	Latest   time.Time
}

type ProcessResult struct {
	Path                 string
	SyoboiTitleID        int
	SyoboiProgramID      int
	EpisodeResolveResult tmdb.EpisodeResolveResult
}

func (p *processor) ProcessMultiple(paths []string, rootDir string, dryRun bool, progress ProgressReporter) ([]ProcessResult, error) {
	recordInfos, err := p.extractInfoAndFilterFiles(paths, progress)
	if err != nil {
		return nil, fmt.Errorf("failed to extract info from files: %w", err)
	}
	if progress != nil {
		progress.Done(fmt.Sprintf("Kept %d files with metadata", len(recordInfos)))
	}

	if progress != nil {
		progress.Start("Grouping recordings into chunks", len(recordInfos))
	}
	pathChunks, err := p.createPathChunks(recordInfos)
	if err != nil {
		return nil, fmt.Errorf("failed to create path chunks: %w", err)
	}
	if progress != nil {
		progress.Done(fmt.Sprintf("Created %d chunks", len(pathChunks)))
	}

	mergedSyoboiResults := make(map[syoboi.Title][]syoboi.ProgramWithRecordInfo)
	if progress != nil {
		progress.Start("Resolving Syoboi programs", len(pathChunks))
	}

	for i, chunk := range pathChunks {
		resolved, err := p.syoboiResolver.Resolve(chunk.Infos, chunk.Earliest, chunk.Latest)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve syoboi programs: %w", err)
		}
		if progress != nil {
			progress.Update(i+1, len(pathChunks), fmt.Sprintf("chunk %d: %d title groups", i+1, len(resolved)))
		}

		for title, programs := range resolved {
			mergedSyoboiResults[title] = append(mergedSyoboiResults[title], programs...)
		}
	}
	if progress != nil {
		progress.Done(fmt.Sprintf("Resolved %d title groups", len(mergedSyoboiResults)))
		progress.Start("Resolving TMDB episodes", len(mergedSyoboiResults))
	}

	programIDToEpisodeResolveResult := make(map[int]*tmdb.EpisodeResolveResult)
	titles := make([]syoboi.Title, 0, len(mergedSyoboiResults))
	for title := range mergedSyoboiResults {
		titles = append(titles, title)
	}
	for i, title := range titles {
		programs := mergedSyoboiResults[title]
		searchTitle := title.Title
		if title.ShortTitle != "" {
			searchTitle = title.ShortTitle
		}
		season, searchTitle := cutSeasonFromSyoboiTitle(searchTitle)

		flag := programs[0].Program.Flag
		// https://docs.cal.syoboi.jp/spec/proginfo-flag/
		isRebroadcast := flag >= 8

		results, err := p.episodeResolver.Resolve(searchTitle, season, isRebroadcast, programs)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve episodes: %w", err)
		}
		maps.Copy(programIDToEpisodeResolveResult, results)
		if progress != nil {
			progress.Update(i+1, len(mergedSyoboiResults), fmt.Sprintf("%s: %d episodes", searchTitle, len(results)))
		}
	}
	if progress != nil {
		progress.Done(fmt.Sprintf("Resolved %d episodes", len(programIDToEpisodeResolveResult)))
		progress.Start("Building rename targets", len(mergedSyoboiResults))
	}

	var processResults []ProcessResult
	for _, programs := range mergedSyoboiResults {
		for _, program := range programs {
			episodeResolveResult, ok := programIDToEpisodeResolveResult[program.ID]
			if !ok {
				slog.Warn("No episode resolve result found for program", "programID", program.ID, "title", program.STSubTitle)
				continue
			}

			processResults = append(processResults, ProcessResult{
				Path:                 program.RecordInfo.Path,
				SyoboiTitleID:        program.TitleID,
				SyoboiProgramID:      program.ID,
				EpisodeResolveResult: *episodeResolveResult,
			})
		}
	}
	if progress != nil {
		progress.Done(fmt.Sprintf("Prepared %d rename targets", len(processResults)))
	}

	return processResults, nil
}

func (p *processor) extractInfoAndFilterFiles(paths []string, progress ProgressReporter) ([]record.Info, error) {
	var result []record.Info

	if progress != nil {
		progress.Start("Extracting file metadata", len(paths))
	}
	for i, path := range paths {
		if progress != nil {
			progress.Update(i+1, len(paths), path)
		}
		info, err := p.infoExtractor.Extract(path)
		if err != nil {
			slog.Error("Failed to extract info for file", "path", path, "error", err)
			continue
		}
		result = append(result, info)
	}

	return result, nil
}

func (p *processor) createPathChunks(recordInfos []record.Info) ([]pathChunk, error) {
	slices.SortFunc(recordInfos, func(a, b record.Info) int {
		return a.BroadcastStartTime.Compare(b.BroadcastStartTime)
	})

	var pathChunks []pathChunk
	var currentChunkIndex int
	for i, info := range recordInfos {
		if i > 0 {
			prevInfo := recordInfos[i-1]
			if info.BroadcastStartTime.Sub(prevInfo.BroadcastStartTime) > createNewChunksThreshold {
				currentChunkIndex++
			}
		}

		if len(pathChunks) <= currentChunkIndex {
			pathChunks = append(pathChunks, pathChunk{})
		}

		currentChunk := &pathChunks[currentChunkIndex]
		currentChunk.Infos = append(currentChunk.Infos, info)
		if currentChunk.Earliest.IsZero() || info.BroadcastStartTime.Before(currentChunk.Earliest) {
			currentChunk.Earliest = info.BroadcastStartTime
		}
		endTime := info.BroadcastStartTime.Add(info.BroadcastDuration)
		if currentChunk.Latest.IsZero() || endTime.After(currentChunk.Latest) {
			currentChunk.Latest = endTime
		}
	}
	return pathChunks, nil
}
