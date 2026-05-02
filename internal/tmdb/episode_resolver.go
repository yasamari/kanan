package tmdb

import (
	"fmt"
	"log/slog"
	"maps"
	"time"

	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/yasamari/kanan/internal/syoboi"
	"github.com/yasamari/kanan/internal/util"
)

type EpisodeResolver interface {
	Resolve(title string, seasonNumber int, isRebroadcast bool, programs []syoboi.ProgramWithRecordInfo) (map[int]*EpisodeResolveResult, error)
}

type episodeResolver struct {
	client *tmdb.Client
}

func NewEpisodeResolver(client *tmdb.Client) *episodeResolver {
	return &episodeResolver{
		client: client,
	}
}

type EpisodeResolveResult struct {
	ShowID        int64
	ShowName      string
	FirstAirDate  time.Time
	SeasonNumber  int
	SeasonName    string
	EpisodeName   string
	EpisodeNumber int
}

var urlOptions = map[string]string{
	"language": "ja-JP",
}

const (
	maxTVShowsToProcess = 5

	showNameSimilarityThreshold     = 0.4
	episodeTitleSimilarityThreshold = 0.7

	startTimeBeforeAirDateThreshold = 24 * 6 * time.Hour

	syoboiTimeFormat = "2006-01-02 15:04:05"

	timeZone = "Asia/Tokyo"
)

func (r *episodeResolver) Resolve(title string, seasonNumber int, isRebroadcast bool, programs []syoboi.ProgramWithRecordInfo) (map[int]*EpisodeResolveResult, error) {
	results := make(map[int]*EpisodeResolveResult)

	earliestStartTime, err := getProgramEarliestStartTime(programs)
	if err != nil {
		return results, fmt.Errorf("failed to get earliest start time: %w", err)
	}

	tvShows, err := r.client.GetSearchTVShow(title, urlOptions)
	if err != nil {
		return results, fmt.Errorf("failed to search TV show: %w", err)
	}
	tvShows.Results = tvShows.Results[:min(len(tvShows.Results), maxTVShowsToProcess)]

	for _, tvShow := range tvShows.Results {
		if util.Similarity(tvShow.Name, title) < showNameSimilarityThreshold {
			slog.Debug("Skipping TV show due to low name similarity", "name", tvShow.Name, "similarity", util.Similarity(tvShow.Name, title))
			continue
		}

		tvDetails, err := r.client.GetTVDetails(int(tvShow.ID), urlOptions)
		if err != nil {
			return results, fmt.Errorf("failed to get TV details for show ID %d: %w", tvShow.ID, err)
		}

		var triedSeasonNumber int

		if isRebroadcast && isSeasonNumberExists(tvDetails.Seasons, seasonNumber) {
			triedSeasonNumber = seasonNumber
		} else {
			closestSeasonNumber, err := getCloseAirDateSeasonNumber(earliestStartTime, tvDetails.Seasons)
			if err != nil {
				return results, fmt.Errorf("failed to get closest season number: %w", err)
			}
			triedSeasonNumber = closestSeasonNumber
		}

		if triedSeasonNumber != 0 {
			slog.Debug("Trying season", "showName", tvShow.Name, "seasonNumber", triedSeasonNumber)
			seasonDetails, err := r.client.GetTVSeasonDetails(int(tvShow.ID), triedSeasonNumber, urlOptions)
			if err != nil {
				return results, fmt.Errorf("failed to get season details for show ID %d season %d: %w", tvShow.ID, triedSeasonNumber, err)
			}
			resolved, err := r.findTVSeasonDetails(programs, isRebroadcast, tvDetails, seasonDetails)
			if err != nil {
				return results, fmt.Errorf("failed to find TV season details: %w", err)
			}
			maps.Copy(results, resolved)

			if len(results) >= len(programs) {
				slog.Debug("All programs resolved", "resolvedCount", len(results), "programCount", len(programs))
				break
			}
		}

		if len(results) < len(programs) {
			// すべての番組が解決できなかった場合、他のすべてのシーズンを試す
			for _, season := range tvDetails.Seasons {
				// 特別編またはすでに試したシーズンはスキップ
				if season.SeasonNumber == 0 || season.SeasonNumber == triedSeasonNumber {
					continue
				}

				slog.Debug("Trying another season", "showName", tvShow.Name, "seasonNumber", season.SeasonNumber)

				seasonDetails, err := r.client.GetTVSeasonDetails(int(tvShow.ID), season.SeasonNumber, urlOptions)
				if err != nil {
					slog.Debug("Failed to get season details, skipping", "showID", tvShow.ID, "seasonNumber", season.SeasonNumber, "error", err)
					continue
				}
				resolved, err := r.findTVSeasonDetails(programs, isRebroadcast, tvDetails, seasonDetails)
				if err != nil {
					slog.Debug("Failed to find TV season details, skipping", "showID", tvShow.ID, "seasonNumber", season.SeasonNumber, "error", err)
					continue
				}
				maps.Copy(results, resolved)

				if len(results) >= len(programs) {
					slog.Debug("All programs resolved", "resolvedCount", len(results), "programCount", len(programs))
					break
				}

				slog.Debug("Not all programs resolved for this show, trying next show", "showID", tvShow.ID, "resolvedCount", len(results), "programCount", len(programs))
			}
		}

		if len(results) >= len(programs) {
			break
		}
	}

	if len(results) < len(programs) {
		slog.Warn("Not all programs could be resolved", "resolvedCount", len(results), "programCount", len(programs))
	}

	return results, nil
}

func (r *episodeResolver) findTVSeasonDetails(programs []syoboi.ProgramWithRecordInfo, isRebroadcast bool, tvDetails *tmdb.TVDetails, seasonDetails *tmdb.TVSeasonDetails) (map[int]*EpisodeResolveResult, error) {
	results := make(map[int]*EpisodeResolveResult)

	loc, _ := time.LoadLocation(timeZone)
	firstAirDate, err := time.ParseInLocation(time.DateOnly, tvDetails.FirstAirDate, loc)
	if err != nil {
		if len(tvDetails.FirstAirDate) == 4 {
			firstAirDate, err = time.ParseInLocation("2006", tvDetails.FirstAirDate, loc)
			if err != nil {
				return results, fmt.Errorf("failed to parse first air date with year-only format: %w", err)
			}
		}
		return results, fmt.Errorf("failed to parse first air date: %w", err)
	}

	for _, program := range programs {
		for _, episode := range seasonDetails.Episodes {
			if isRebroadcast {
				if util.Similarity(program.STSubTitle, episode.Name) < episodeTitleSimilarityThreshold {
					continue
				}
			} else {
				programStartTime, err := time.ParseInLocation(syoboiTimeFormat, program.StartTime, loc)
				if err != nil {
					slog.Debug("Failed to parse program start time, skipping rebroadcast check", "programID", program.ID, "startTime", program.StartTime, "error", err)
					break
				}
				episodeAirDate, err := time.ParseInLocation(time.DateOnly, episode.AirDate, loc)
				if err != nil {
					slog.Debug("Failed to parse episode air date, skipping rebroadcast check", "showID", tvDetails.ID, "seasonNumber", seasonDetails.SeasonNumber, "episodeNumber", episode.EpisodeNumber, "airDate", episode.AirDate, "error", err)
					break
				}

				if programStartTime.Before(episodeAirDate) ||
					programStartTime.Sub(episodeAirDate).Abs() > startTimeBeforeAirDateThreshold {
					continue
				}

				match := program.Count == episode.EpisodeNumber || util.Similarity(program.STSubTitle, episode.Name) >= episodeTitleSimilarityThreshold
				if !match {
					continue
				}
			}

			results[program.ID] = &EpisodeResolveResult{
				ShowID:        tvDetails.ID,
				ShowName:      tvDetails.Name,
				FirstAirDate:  firstAirDate,
				SeasonName:    seasonDetails.Name,
				SeasonNumber:  seasonDetails.SeasonNumber,
				EpisodeName:   episode.Name,
				EpisodeNumber: episode.EpisodeNumber,
			}
		}
	}

	return results, nil
}

func isSeasonNumberExists(seasons []tmdb.Season, seasonNumber int) bool {
	for _, season := range seasons {
		if season.SeasonNumber == seasonNumber {
			return true
		}
	}
	return false
}

func getProgramEarliestStartTime(programs []syoboi.ProgramWithRecordInfo) (time.Time, error) {
	var earliest time.Time

	loc, _ := time.LoadLocation(timeZone)

	for _, program := range programs {
		startTime, err := time.ParseInLocation(syoboiTimeFormat, program.StartTime, loc)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse program start time: %w", err)
		}

		if earliest.IsZero() || startTime.Before(earliest) {
			earliest = startTime
		}
	}
	return earliest, nil
}

func getCloseAirDateSeasonNumber(airDate time.Time, seasons []tmdb.Season) (int, error) {
	var closestSeasonNumber int
	var closestDiff time.Duration

	loc, _ := time.LoadLocation(timeZone)

	for _, season := range seasons {
		if season.AirDate == "" {
			continue
		}

		seasonAirDate, err := time.ParseInLocation(time.DateOnly, season.AirDate, loc)
		if err != nil {
			if len(season.AirDate) == 4 {
				seasonAirDate, err = time.ParseInLocation("2006", season.AirDate, loc)
				if err != nil {
					slog.Debug("Failed to parse season air date with year-only format, skipping", "seasonNumber", season.SeasonNumber, "airDate", season.AirDate)
					continue
				}
			} else {
				slog.Debug("Failed to parse season air date, skipping", "seasonNumber", season.SeasonNumber, "airDate", season.AirDate)
				continue
			}
			continue
		}

		diff := seasonAirDate.Sub(airDate).Abs()
		if closestSeasonNumber == 0 || diff < closestDiff {
			closestSeasonNumber = season.SeasonNumber
			closestDiff = diff
		}
	}

	return closestSeasonNumber, nil
}
