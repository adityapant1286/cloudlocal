package cloudwatch

import "sync"

//type cloudwatchServiceImplementation struct {
//	cloudwatch CwService
//}

type LogEvent struct {
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
}

type LogStream struct {
	Events []LogEvent
}

type LogGroupRes struct {
	LogGroupName string `json:"logGroupName"`
}

type LogStreamRes struct {
	LogStreamName string `json:"logStreamName"`
}

type cloudwatchImplementation struct {
	mu          sync.RWMutex
	Groups      map[string]map[string]*LogStream // GroupName -> StreamName -> Stream
	storagePath string
}

type persistentState struct {
	Groups map[string]map[string]*LogStream `json:"groups"`
}
