package processor

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/yasamari/kanan/internal/record"
	"github.com/yasamari/kanan/internal/saya"
	"github.com/yasamari/kanan/internal/syoboi"
)

type SyoboiProgram struct {
	ID        int
	TitleID   int
	ChannelID int
	Title     string
	SubTitle  string
	Season    *int
	Episode   int
}

func (p *Processor) getProgramFromSyoboi(info record.Info) (*SyoboiProgram, error) {
	channelID, err := saya.GetSyoboiChannelIDByServiceID(info.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Syoboi channel ID: %w", err)
	}

	// 240000だと404になるため、常にdurationから1秒引いて検索する
	endTime := info.StartTime.Add(info.Duration - time.Second)
	programs, err := p.syoboiClient.SearchProgramsByChannelAndTime(channelID, info.StartTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to search programs: %w", err)
	}

	program, err := filterSyoboiPrograms(programs, info)
	if err != nil {
		return nil, fmt.Errorf("failed to filter programs: %w", err)
	}

	programTitle, err := p.syoboiClient.GetTitleByID(int64(program.TitleID))
	if err != nil {
		return nil, fmt.Errorf("failed to get title: %w", err)
	}
	title := programTitle.Title

	// 短いタイトルのほうがシーズンを抽出しやすいので優先する
	if programTitle.ShortTitle != "" {
		title = programTitle.ShortTitle
	}

	season, title := cutSeasonFromSyoboiTitle(title)

	return &SyoboiProgram{
		ID:        program.ID,
		TitleID:   program.TitleID,
		ChannelID: channelID,
		Title:     title,
		SubTitle:  program.STSubTitle,
		Episode:   program.Count,
		Season:    season,
	}, nil
}

const stTimeFormat = "2006-01-02 15:04:05"

func filterSyoboiPrograms(programs []syoboi.Program, info record.Info) (*syoboi.Program, error) {
	var filtered []syoboi.Program

	loc, _ := time.LoadLocation("Asia/Tokyo")

	for _, p := range programs {
		startTime, err := time.ParseInLocation(stTimeFormat, p.StartTime, loc)
		if err != nil {
			continue
		}
		endTime, err := time.ParseInLocation(stTimeFormat, p.EndTime, loc)
		if err != nil {
			continue
		}

		if startTime.Equal(info.StartTime) && endTime.Equal(info.StartTime.Add(info.Duration)) {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) == 0 || len(filtered) > 1 {
		return nil, fmt.Errorf("program not found")
	}
	return &filtered[0], nil
}

var (
	seasonRegex       = regexp.MustCompile(`\((\d+)\)$`)
	seasonSuffixRegex = regexp.MustCompile(`[sS]eason\s*(\d+)$`)
)

func cutSeasonFromSyoboiTitle(title string) (*int, string) {
	// タイトルの末尾に "(X)" の形式でシーズン表記があるか確認する
	if seasonRegex.MatchString(title) {
		seasonNum, err := strconv.Atoi(seasonRegex.FindStringSubmatch(title)[1])
		removed := seasonRegex.ReplaceAllString(title, "")
		if err != nil {
			return nil, title
		}
		return &seasonNum, removed
	}

	// タイトルの末尾に "Season X" があるか確認する
	if seasonSuffixRegex.MatchString(title) {
		seasonNum, err := strconv.Atoi(seasonSuffixRegex.FindStringSubmatch(title)[1])
		removed := seasonSuffixRegex.ReplaceAllString(title, "")
		if err != nil {
			return nil, title
		}
		return &seasonNum, removed
	}
	return nil, title
}
