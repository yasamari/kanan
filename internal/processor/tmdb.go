package processor

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/yasamari/kanan/internal/util"
)

var (
	urlOptions = map[string]string{
		"language": "ja-JP",
	}

	ErrNotFound = errors.New("not found")
)

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

func (p *Processor) getTmdbInfo(syoboiInfo syoboiInfo) (*tmdbInfo, error) {
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

		if syoboiInfo.Season != nil && details.NumberOfSeasons < *syoboiInfo.Season {

		}

		firstAirDate, err := time.Parse(time.DateOnly, show.FirstAirDate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse first air date: %w", err)
		}

		for _, season := range details.Seasons {
			// Skip specials
			if season.SeasonNumber == 0 {
				continue
			}

			episodeID, episodeNumber, episodeTitle, err := p.searchTmdbEpisode(show.ID, season.SeasonNumber, syoboiInfo)
			if err == nil {
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
			if !errors.Is(err, ErrNotFound) {
				return nil, fmt.Errorf("failed to search TMDB episode: %w", err)
			}
		}
	}

	return nil, ErrNotFound
}

func (p *Processor) searchTmdbEpisode(showID int64, seasonNumber int, syoboiInfo syoboiInfo) (int64, int, string, error) {
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

	for _, episode := range detail.Episodes {
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
