package processor

import (
	"fmt"
	"path/filepath"

	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/yasamari/kanan/internal/record"
	"github.com/yasamari/kanan/internal/syoboi"
)

type processor struct {
	syoboiClient  syoboi.Client
	tmdbClient    *tmdb.Client
	infoExtractor record.InfoExtractor
}

func New(syoboiClient syoboi.Client, tmdbClient *tmdb.Client, infoExtractor record.InfoExtractor) *processor {
	return &processor{
		syoboiClient:  syoboiClient,
		tmdbClient:    tmdbClient,
		infoExtractor: infoExtractor,
	}
}

const (
	seriesDirFormat   = "%s (%d) [tmdbid-%d]"
	seasonDirFormat   = "Season %02d [syobocalid-%d]"
	episodeFileFormat = "%s S%02dE%02d [syobocalid-%d].%s"
)

func (p *processor) Process(path string) error {
	recordFileInfo, err := p.infoExtractor.Extract(path)
	if err != nil {
		return fmt.Errorf("failed to extract broadcast info: %w", err)
	}

	syoboiInfo, err := p.getProgramInfoFromSyoboi(recordFileInfo)
	if err != nil {
		return fmt.Errorf("failed to get program from Syoboi: %w", err)
	}

	fmt.Printf("Syoboi Program: %+v\n", syoboiInfo)

	tmdbInfo, err := p.getTmdbInfo(syoboiInfo)
	if err != nil {
		return fmt.Errorf("failed to get TMDB Info: %w", err)
	}

	fmt.Printf("TMDB Info: %+v\n", tmdbInfo)
	finalPath := makePath(filepath.Dir(path), tmdbInfo, syoboiInfo)
	fmt.Printf("Final Path: %s\n", finalPath)

	return nil
}

func makePath(rootDir string, tmdbInfo *tmdbInfo, syoboiInfo syoboiInfo) string {
	seriesDir := fmt.Sprintf(seriesDirFormat, tmdbInfo.ShowName, tmdbInfo.ShowFirstAirDate.Year(), tmdbInfo.ShowID)
	seasonDir := fmt.Sprintf(seasonDirFormat, tmdbInfo.SeasonNumber, syoboiInfo.TitleID)
	episodeFile := fmt.Sprintf(episodeFileFormat, tmdbInfo.EpisodeTitle, tmdbInfo.SeasonNumber, tmdbInfo.EpisodeNumber, syoboiInfo.ID, "ts")

	return filepath.Join(rootDir, seriesDir, seasonDir, episodeFile)
}
