package record

import (
	"fmt"
	"time"

	"github.com/koorimizuw/aribtool/tsparser"
)

type tsExtractor struct{}

var _ InfoExtractor = (*tsExtractor)(nil)

func NewTsInfoExtractor() *tsExtractor {
	return &tsExtractor{}
}

func (e *tsExtractor) Extract(path string) (Info, error) {
	patPid := tsparser.PidMap[tsparser.ProgramAssociationSection]
	patTidRange := tsparser.TableIdMap[tsparser.ProgramAssociationSection]
	patSectionList := tsparser.Scan(path, patPid, patTidRange, 100)
	if len(patSectionList) == 0 {
		return Info{}, fmt.Errorf("PAT section not found")
	}
	sid := tsparser.GetSid(patSectionList[0])

	eventPid := tsparser.PidMap[tsparser.CurrentEventSection]
	eventTidRange := tsparser.TableIdMap[tsparser.CurrentEventSection]
	eventSectionList := tsparser.Scan(path, eventPid, eventTidRange, 100)
	events := tsparser.ParseCurrentEventSection(sid, 0, eventSectionList...)
	if len(events) == 0 {
		return Info{}, fmt.Errorf("event not found for sid: %d", sid)
	}

	event := events[len(events)-1]

	return Info{
		Path:               path,
		BroadcastStartTime: event.StartTime,
		BroadcastDuration:  time.Duration(event.Duration) * time.Minute,
		ServiceID:          event.ServiceId,
	}, nil
}
