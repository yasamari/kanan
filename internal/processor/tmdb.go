package processor

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/yasamari/kanan/internal/util"
)

var urlOptions = map[string]string{
	"language": "ja-JP",
}

const (
	showTitleSimilarityThreshold    = 0.4
	episodeTitleSimilarityThreshold = 0.7

	showMaxProcessed = 5
)

type tmdbInfo struct {
	ShowID           int64
	ShowName         string
	ShowFirstAirDate time.Time
	SeasonID         int64
	SeasonNumber     int
	EpisodeID        int64
	EpisodeNumber    int
	EpisodeTitle     string
}

func (p *processor) getTmdbInfo(syoboiInfo syoboiInfo) (*tmdbInfo, error) {
	shows, err := p.tmdbClient.GetSearchTVShow(syoboiInfo.Title, urlOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to search TV show: %w", err)
	}

	slog.Debug("Found TV shows", "count", len(shows.Results))

	processed := 0

	for _, show := range shows.Results {
		if util.Similarity(show.Name, syoboiInfo.Title) < showTitleSimilarityThreshold {
			continue
		}

		processed++
		if processed > showMaxProcessed {
			break
		}

		slog.Debug("Checking TMDB show", "name", show.Name, "id", show.ID)

		details, err := p.tmdbClient.GetTVDetails(int(show.ID), urlOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to get TV details: %w", err)
		}

		if len(details.Seasons) == 0 {
			slog.Debug("No seasons found for show, skipping", "showID", show.ID)
			continue
		}

		var (
			airDateCloseSeason     *tmdb.Season
			airDateCloseSeasonDiff time.Duration
		)

		if !syoboiInfo.Rebroadcast {
			for _, season := range details.Seasons {
				if season.SeasonNumber == 0 {
					continue
				}

				seasonAirDate, err := time.Parse(time.DateOnly, season.AirDate)
				if err != nil && len(season.AirDate) == 4 {
					seasonAirDate, err = time.Parse("2006", season.AirDate)
					if err != nil {
						slog.Debug("Failed to parse season air date, skipping", "showID", show.ID, "seasonNumber", season.SeasonNumber, "airDate", season.AirDate)
						continue
					}
				}

				diff := seasonAirDate.Sub(syoboiInfo.StartTime).Abs()

				if airDateCloseSeason == nil || diff < airDateCloseSeasonDiff {
					airDateCloseSeason = &season
					airDateCloseSeasonDiff = diff
				}
			}
		}

		if airDateCloseSeason != nil {
			episodeID, episodeNumber, episodeTitle, err := p.searchTmdbEpisode(show.ID, airDateCloseSeason.SeasonNumber, syoboiInfo)
			if err == nil {
				slog.Debug("Found episode in air date close season", "showID", show.ID, "seasonNumber", airDateCloseSeason.SeasonNumber, "episodeNumber", episodeNumber)
				return createTmdbInfoFromSearchResult(show, *airDateCloseSeason, episodeID, episodeNumber, episodeTitle)
			}
			if !errors.Is(err, ErrNotFound) {
				return nil, fmt.Errorf("failed to search TMDB episode: %w", err)
			}
		}

		for _, season := range details.Seasons {
			// Skip specials
			if season.SeasonNumber == 0 {
				slog.Debug("Skipping special season", "showID", show.ID, "seasonName", season.Name)
				continue
			}

			if airDateCloseSeason != nil && season.SeasonNumber == airDateCloseSeason.SeasonNumber {
				slog.Debug("Already tried this season, skipping", "showID", show.ID, "seasonNumber", season.SeasonNumber)
				continue
			}

			episodeID, episodeNumber, episodeTitle, err := p.searchTmdbEpisode(show.ID, season.SeasonNumber, syoboiInfo)
			if err == nil {
				return createTmdbInfoFromSearchResult(show, season, episodeID, episodeNumber, episodeTitle)
			}
			if !errors.Is(err, ErrNotFound) {
				return nil, fmt.Errorf("failed to search TMDB episode: %w", err)
			}
		}
	}

	return nil, ErrNotFound
}

func createTmdbInfoFromSearchResult(show tmdb.TVShowResult, season tmdb.Season, episodeID int64, episodeNumber int, episodeTitle string) (*tmdbInfo, error) {
	firstAirDate, err := time.Parse(time.DateOnly, show.FirstAirDate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse first air date: %w", err)
	}

	return &tmdbInfo{
		ShowID:           show.ID,
		ShowName:         show.Name,
		ShowFirstAirDate: firstAirDate,
		SeasonID:         season.ID,
		SeasonNumber:     season.SeasonNumber,
		EpisodeID:        episodeID,
		EpisodeNumber:    episodeNumber,
		EpisodeTitle:     episodeTitle,
	}, nil
}

func (p *processor) searchTmdbEpisode(showID int64, seasonNumber int, syoboiInfo syoboiInfo) (int64, int, string, error) {
	slog.Debug("Searching TMDB episode", "showID", showID, "seasonNumber", seasonNumber)

	detail, err := p.tmdbClient.GetTVSeasonDetails(int(showID), seasonNumber, urlOptions)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to get TV season details: %w", err)
	}

	if len(detail.Episodes) == 0 {
		return 0, 0, "", ErrNotFound
	}

	topEpisode := detail.Episodes[0]
	var maxSimilarity float64

	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to load location: %w", err)
	}

	for _, episode := range detail.Episodes {
		airDate, err := time.ParseInLocation(time.DateOnly, episode.AirDate, loc)
		if err != nil {
			return 0, 0, "", fmt.Errorf("failed to parse air date: %w", err)
		}

		if !syoboiInfo.Rebroadcast && syoboiInfo.StartTime.Before(airDate) {
			continue
		}

		similarity := util.Similarity(syoboiInfo.SubTitle, episode.Name)
		if similarity > maxSimilarity {
			maxSimilarity = similarity
			topEpisode = episode
		}
	}

	if maxSimilarity >= episodeTitleSimilarityThreshold {
		return topEpisode.ID, topEpisode.EpisodeNumber, topEpisode.Name, nil
	}

	return 0, 0, "", ErrNotFound
}
