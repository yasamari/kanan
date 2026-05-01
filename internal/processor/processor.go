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

func (p *processor) ProcessMultiple(paths []string, rootDir string, dryRun bool) ([]ProcessResult, error) {
	slog.Info("Extracting info from files", "count", len(paths))
	recordInfos, err := p.extractInfoAndFilterFiles(paths)
	if err != nil {
		return nil, fmt.Errorf("failed to extract info from files: %w", err)
	}
	slog.Info("Finished extracting info from files", "count", len(recordInfos))

	pathChunks, err := p.createPathChunks(recordInfos)
	if err != nil {
		return nil, fmt.Errorf("failed to create path chunks: %w", err)
	}
	slog.Info("Created file chunks", "chunkCount", len(pathChunks), "totalFiles", len(paths))

	mergedSyoboiResults := make(map[syoboi.Title][]syoboi.ProgramWithRecordInfo)

	for i, chunk := range pathChunks {
		resolved, err := p.syoboiResolver.Resolve(chunk.Infos, chunk.Earliest, chunk.Latest)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve syoboi programs: %w", err)
		}
		slog.Info("Finished resolving syoboi programs for chunk", "chunkIndex", i, "matchedPrograms", len(resolved))

		for title, programs := range resolved {
			mergedSyoboiResults[title] = append(mergedSyoboiResults[title], programs...)
		}
	}

	programIDToEpisodeResolveResult := make(map[int]*tmdb.EpisodeResolveResult)
	for title, programs := range mergedSyoboiResults {
		searchTitle := title.Title
		if title.ShortTitle != "" {
			searchTitle = title.ShortTitle
		}
		season, searchTitle := cutSeasonFromSyoboiTitle(searchTitle)

		isRebroadcast := programs[0].Program.Flag == 8 || programs[0].Program.Flag == 10

		results, err := p.episodeResolver.Resolve(searchTitle, season, isRebroadcast, programs)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve episodes: %w", err)
		}
		maps.Copy(programIDToEpisodeResolveResult, results)
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

	return processResults, nil
}

func (p *processor) extractInfoAndFilterFiles(paths []string) ([]record.Info, error) {
	var result []record.Info

	for _, path := range paths {
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
