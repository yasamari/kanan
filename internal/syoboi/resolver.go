package syoboi

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/yasamari/kanan/internal/record"
)

type Resolver interface {
	Resolve(recordInfos []record.Info, earliest, latest time.Time) (map[Title][]ProgramWithRecordInfo, error)
}

type resolver struct {
	client               Client
	serviceIDToChannelID map[int]int
}

func NewResolver(client Client, serviceIDToChannelID map[int]int) *resolver {
	return &resolver{
		client:               client,
		serviceIDToChannelID: serviceIDToChannelID,
	}
}

type ProgramWithRecordInfo struct {
	Program
	RecordInfo record.Info
}

const syoboiTimeFormat = "2006-01-02 15:04:05"

func (r *resolver) Resolve(recordInfos []record.Info, earliest, latest time.Time) (map[Title][]ProgramWithRecordInfo, error) {
	channelIDs := infosToChannelIDs(recordInfos, r.serviceIDToChannelID)
	searchEarliest := earliest
	searchInfos := recordInfos

	titleIDsMap := make(map[int]struct{})
	var programWithRecordInfos []ProgramWithRecordInfo

	loc, _ := time.LoadLocation("Asia/Tokyo")
	for {
		slog.Debug("Searching syoboi programs", "channelIDs", channelIDs, "startTime", searchEarliest, "endTime", latest)
		programs, err := r.client.SearchProgramsByChannelAndTime(channelIDs, searchEarliest, latest.Add(-1*time.Second))
		if err != nil {
			return nil, fmt.Errorf("failed to search programs: %w", err)
		}

		slog.Debug("Found programs", "count", len(programs))

		time.Sleep(1 * time.Second)

		for _, prog := range programs {
			if prog.Deleted != 0 {
				continue
			}

			endTime, err := time.ParseInLocation(syoboiTimeFormat, prog.EndTime, loc)
			if err != nil {
				slog.Error("Failed to parse end time", "endTime", prog.EndTime, "error", err)
				continue
			}
			if endTime.After(searchEarliest) {
				searchEarliest = endTime
			}
			startTime, err := time.ParseInLocation(syoboiTimeFormat, prog.StartTime, loc)
			if err != nil {
				slog.Error("Failed to parse start time", "startTime", prog.StartTime, "error", err)
				continue
			}

			for _, info := range searchInfos {
				// チャンネルID、開始時間、終了時間が一致するものをマッチとする
				channelID, ok := r.serviceIDToChannelID[info.ServiceID]
				if !ok {
					slog.Debug("No channel ID found for service ID, skipping", "serviceID", info.ServiceID)
					continue
				}

				if channelID == prog.ChannelID &&
					startTime.Equal(info.BroadcastStartTime) &&
					endTime.Equal(info.BroadcastStartTime.Add(info.BroadcastDuration)) {
					programWithRecordInfos = append(programWithRecordInfos, ProgramWithRecordInfo{
						Program:    prog,
						RecordInfo: info,
					})

					titleIDsMap[prog.TitleID] = struct{}{}

					break
				}
			}
		}

		var filteredInfos []record.Info
		for _, info := range searchInfos {
			if info.BroadcastStartTime.After(searchEarliest) {
				filteredInfos = append(filteredInfos, info)
			}
		}
		searchInfos = filteredInfos
		channelIDs = infosToChannelIDs(searchInfos, r.serviceIDToChannelID)

		slog.Debug("Finished processing programs, updating search infos", "remainingSearchInfos", len(searchInfos))

		if len(programs) == 0 {
			slog.Debug("No more programs found, finishing resolution")
			break
		}

		if len(searchInfos) == 0 {
			slog.Debug("No more search infos remaining, finishing resolution")
			break
		}
	}

	var titleIDs []int
	for titleID := range titleIDsMap {
		titleIDs = append(titleIDs, titleID)
	}

	slog.Debug("Fetching titles for matched programs", "titleIDs", titleIDs)
	titles, err := r.client.GetTitleByIDs(titleIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get titles: %w", err)
	}
	titleIDToTitle := make(map[int]Title)
	for _, title := range titles {
		titleIDToTitle[title.ID] = title
	}

	result := make(map[Title][]ProgramWithRecordInfo)

	for _, p := range programWithRecordInfos {
		title, ok := titleIDToTitle[p.Program.TitleID]
		if !ok {
			slog.Debug("No title found for title ID, skipping", "titleID", p.Program.TitleID)
			continue
		}
		result[title] = append(result[title], p)
	}
	return result, nil
}

func infosToChannelIDs(infos []record.Info, serviceIDToChannelID map[int]int) []int {
	channelIDsMap := make(map[int]struct{})
	for _, info := range infos {
		channelID, ok := serviceIDToChannelID[info.ServiceID]
		if !ok {
			slog.Debug("No channel ID found for service ID, skipping", "serviceID", info.ServiceID)
			continue
		}
		channelIDsMap[channelID] = struct{}{}
	}

	var channelIDs []int
	for channelID := range channelIDsMap {
		channelIDs = append(channelIDs, channelID)
	}

	return channelIDs
}
