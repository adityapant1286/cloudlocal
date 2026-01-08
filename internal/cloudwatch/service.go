package cloudwatch

import (
	"cloudlocal/internal/utils"
	"encoding/json"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

var (
	once     sync.Once
	instance *cloudwatchImplementation
)

type CwService interface {
	AddLog(group, stream, message string)
	Debug(group, stream, message string)
	Error(group, stream, message string)
	Info(group, stream, message string)
	Warn(group, stream, message string)
	ListGroupNames() []LogGroupRes
	ListLogStreamsNames(group string) []LogStreamRes
	ListEvents(group, stream string) []LogEvent
}

func GetServiceInstance() CwService {
	once.Do(func() {
		cloudwatchDir := filepath.Join(utils.VolumeDir, "cloudwatch")
		path := filepath.Join(cloudwatchDir, "cloudwatch_state.json")

		if err := os.MkdirAll(cloudwatchDir, 0755); err != nil {
			log.Fatalf("Critical: Could not create CloudWatch directory: %v", err)
		}

		instance = &cloudwatchImplementation{
			Groups:      make(map[string]map[string]*LogStream),
			storagePath: path,
		}
		instance.load()
	})
	return instance
}

func (s *cloudwatchImplementation) save() {
	state := persistentState{
		Groups: s.Groups,
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	err := os.WriteFile(s.storagePath, data, 0644)
	if err != nil {
		log.Printf("Error saving CloudWatch state: %s", err)
	}
}

func (s *cloudwatchImplementation) load() {
	data, err := os.ReadFile(s.storagePath)
	if err != nil {
		return // File doesn't exist yet, which is fine
	}
	var state persistentState
	if err := utils.UnmarshalJsonErrors(data, &state); err == nil {
		s.Groups = state.Groups
	} else {
		log.Fatalf("Error loading CloudWatch state: %s", err)
	}
}

func (s *cloudwatchImplementation) Debug(group, stream, message string) {
	s.AddLog(group, stream, "[DEBUG] "+message)
}

func (s *cloudwatchImplementation) Error(group, stream, message string) {
	s.AddLog(group, stream, "[ERROR] "+message)
}

func (s *cloudwatchImplementation) Info(group, stream, message string) {
	s.AddLog(group, stream, "[INFO] "+message)
}

func (s *cloudwatchImplementation) Warn(group, stream, message string) {
	s.AddLog(group, stream, "[WARN] "+message)
}

func (s *cloudwatchImplementation) AddLog(group, stream, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.Groups[group]; !ok {
		s.Groups[group] = make(map[string]*LogStream)
	}
	if _, ok := s.Groups[group][stream]; !ok {
		s.Groups[group][stream] = &LogStream{}
	}

	s.Groups[group][stream].Events = append(s.Groups[group][stream].Events, LogEvent{
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
		Message:   message,
	})
	s.save()
}

func (s *cloudwatchImplementation) ListGroupNames() []LogGroupRes {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]LogGroupRes, 0, len(s.Groups))
	for _, g := range slices.Collect(maps.Keys(s.Groups)) {
		res = append(res, LogGroupRes{LogGroupName: g})
	}
	return res
}

func (s *cloudwatchImplementation) ListLogStreamsNames(group string) []LogStreamRes {
	s.mu.RLock()
	defer s.mu.RUnlock()

	gstreams := s.Groups[group]

	if gstreams != nil {
		res := make([]LogStreamRes, 0, len(gstreams))

		for _, s := range slices.Collect(maps.Keys(gstreams)) {
			res = append(res, LogStreamRes{LogStreamName: s})
		}
		return res
	}

	return []LogStreamRes{}

}

func (s *cloudwatchImplementation) ListEvents(group, stream string) []LogEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g := s.Groups[group]

	if g != nil {
		logStream := g[stream]
		if logStream != nil {
			return logStream.Events
		}
	}

	return []LogEvent{}
}
