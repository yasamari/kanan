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

type matchKey struct {
	channelID int
	start     int64
	end       int64
}

func (r *resolver) Resolve(recordInfos []record.Info, earliest, latest time.Time) (map[Title][]ProgramWithRecordInfo, error) {
	searchEarliest := earliest

	infoMap := r.createInfoMap(recordInfos)
	channelIDs := infoMapToChannelIDs(infoMap, r.serviceIDToChannelID)

	titleIDsMap := make(map[int]struct{})
	var programWithRecordInfos []ProgramWithRecordInfo

	loc, _ := time.LoadLocation("Asia/Tokyo")
	for {
		slog.Debug("Searching syoboi programs", "channelIDs", channelIDs, "startTime", searchEarliest, "endTime", latest)
		programs, err := r.client.SearchProgramsByChannelAndTime(channelIDs, searchEarliest, latest.Add(-1*time.Second))
		if err != nil {
			return nil, fmt.Errorf("failed to search programs: %w", err)
		}

		if len(programs) == 0 {
			slog.Debug("No more programs found, finishing resolution")
			break
		}

		slog.Debug("Found programs", "count", len(programs))

		matched := r.matchPrograms(programs, titleIDsMap, infoMap)
		programWithRecordInfos = append(programWithRecordInfos, matched...)

		slog.Debug("Finished processing programs, updating search infos", "remainingSearchInfos", len(infoMap))

		endTime, err := time.ParseInLocation(syoboiTimeFormat, programs[len(programs)-1].EndTime, loc)
		if err != nil {
			slog.Error("Failed to parse end time of last program", "endTime", programs[len(programs)-1].EndTime, "error", err)
			break
		}
		if endTime.After(searchEarliest) {
			searchEarliest = endTime
		}

		for k := range infoMap {
			if k.end <= searchEarliest.Unix() {
				delete(infoMap, k)
			}
		}

		if len(infoMap) == 0 {
			slog.Debug("No more search infos remaining, finishing resolution")
			break
		}

		channelIDs = infoMapToChannelIDs(infoMap, r.serviceIDToChannelID)
	}

	return r.groupByTitle(programWithRecordInfos, titleIDsMap), nil
}

func (r *resolver) matchPrograms(programs []Program, titleIDsMap map[int]struct{}, infoMap map[matchKey]record.Info) []ProgramWithRecordInfo {
	loc, _ := time.LoadLocation("Asia/Tokyo")

	var matched []ProgramWithRecordInfo
	for _, prog := range programs {
		if prog.Deleted != 0 {
			continue
		}

		endTime, err := time.ParseInLocation(syoboiTimeFormat, prog.EndTime, loc)
		if err != nil {
			slog.Error("Failed to parse end time", "endTime", prog.EndTime, "error", err)
			continue
		}

		startTime, err := time.ParseInLocation(syoboiTimeFormat, prog.StartTime, loc)
		if err != nil {
			slog.Error("Failed to parse start time", "startTime", prog.StartTime, "error", err)
			continue
		}

		k := matchKey{
			channelID: prog.ChannelID,
			start:     startTime.Unix(),
			end:       endTime.Unix(),
		}

		if info, ok := infoMap[k]; ok {
			matched = append(matched, ProgramWithRecordInfo{
				Program:    prog,
				RecordInfo: info,
			})

			delete(infoMap, k)
			titleIDsMap[prog.TitleID] = struct{}{}
		}
	}
	return matched
}

func (r *resolver) createInfoMap(recordInfos []record.Info) map[matchKey]record.Info {
	infoMap := make(map[matchKey]record.Info, len(recordInfos))

	for _, info := range recordInfos {
		channelID, ok := r.serviceIDToChannelID[info.ServiceID]
		if !ok {
			slog.Debug("No channel ID found for service ID, skipping", "serviceID", info.ServiceID)
			continue
		}

		start := info.BroadcastStartTime
		end := info.BroadcastStartTime.Add(info.BroadcastDuration)

		k := matchKey{
			channelID: channelID,
			start:     start.Unix(),
			end:       end.Unix(),
		}

		infoMap[k] = info
	}
	return infoMap
}

func (r *resolver) groupByTitle(programs []ProgramWithRecordInfo, titleIDsMap map[int]struct{}) map[Title][]ProgramWithRecordInfo {
	titleIDs := make([]int, 0, len(titleIDsMap))
	for titleID := range titleIDsMap {
		titleIDs = append(titleIDs, titleID)
	}

	slog.Debug("Fetching titles for matched programs", "titleIDs", titleIDs)
	titles, err := r.client.GetTitleByIDs(titleIDs)
	if err != nil {
		slog.Error("Failed to get titles", "error", err)
		return nil
	}
	titleIDToTitle := make(map[int]Title)
	for _, title := range titles {
		titleIDToTitle[title.ID] = title
	}

	result := make(map[Title][]ProgramWithRecordInfo)
	for _, p := range programs {
		title, ok := titleIDToTitle[p.Program.TitleID]
		if !ok {
			slog.Debug("No title found for title ID, skipping", "titleID", p.Program.TitleID)
			continue
		}
		result[title] = append(result[title], p)
	}
	return result
}

func infoMapToChannelIDs(infoMap map[matchKey]record.Info, serviceIDToChannelID map[int]int) []int {
	channelIDsMap := make(map[int]struct{})
	for _, info := range infoMap {
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
