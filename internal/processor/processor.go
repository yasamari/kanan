package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	episodeFileFormat = "%s S%02dE%02d [syobocalid-%d]%s"
)

func (p *processor) Process(path string, rootDir string, dryRun bool) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	recordFileInfo, err := p.infoExtractor.Extract(path)
	if err != nil {
		return fmt.Errorf("failed to extract broadcast info: %w", err)
	}
	fmt.Printf("Extracted info: ServiceID=%d, StartTime=%s, Duration=%s\n", recordFileInfo.ServiceID, recordFileInfo.StartTime.Format(time.DateTime), recordFileInfo.Duration)

	syoboiInfo, err := p.getProgramInfoFromSyoboi(recordFileInfo)
	if err != nil {
		return fmt.Errorf("failed to get program from Syoboi: %w", err)
	}
	fmt.Printf("Matched Syoboi calendar program: %s #%d %s\n", syoboiInfo.Title, syoboiInfo.Episode, syoboiInfo.SubTitle)

	tmdbInfo, err := p.getTmdbInfo(syoboiInfo)
	if err != nil {
		return fmt.Errorf("failed to get TMDB Info: %w", err)
	}

	fmt.Printf("Matched TMDB show: %s (Season %d) #%d %s\n", tmdbInfo.ShowName, tmdbInfo.SeasonNumber, tmdbInfo.EpisodeNumber, tmdbInfo.EpisodeTitle)

	fileExt := filepath.Ext(path)

	seriesDir := fmt.Sprintf(seriesDirFormat, tmdbInfo.ShowName, tmdbInfo.ShowFirstAirDate.Year(), tmdbInfo.ShowID)
	seasonDir := fmt.Sprintf(seasonDirFormat, tmdbInfo.SeasonNumber, syoboiInfo.TitleID)
	episodeFile := fmt.Sprintf(episodeFileFormat, tmdbInfo.EpisodeTitle, tmdbInfo.SeasonNumber, tmdbInfo.EpisodeNumber, syoboiInfo.ID, fileExt)

	if !dryRun {
		dirPath := filepath.Join(rootDir, seriesDir, seasonDir)

		err = os.MkdirAll(dirPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		symlinkPath := filepath.Join(dirPath, episodeFile)

		_, err = os.Stat(symlinkPath)
		if err == nil {
			fmt.Printf("Symlink already exists: %s\n", symlinkPath)
			return nil
		}

		err = os.Symlink(path, symlinkPath)
		if err != nil {
			return fmt.Errorf("failed to create symlink: %w", err)
		}
		fmt.Printf("Created symlink: %s -> %s\n", symlinkPath, path)
	}

	return nil
}
