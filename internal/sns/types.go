package sns

import (
	"cloudlocal/internal/cloudwatch"
	"cloudlocal/internal/sqs"
	"sync"
)

const SNS = "sns"
const SERVICE = SNS + "-service"

type snsServiceImplementation struct {
	sns internalSns
}

type snsImplementation struct {
	mu            sync.RWMutex
	cloudwatch    cloudwatch.CwService
	topics        map[string]bool
	subscriptions map[string][]string // TopicARN -> []QueueURL
	sqsSvc        sqs.ServiceHandler
}

type CreateTopicResponse struct {
	TopicArn  string `json:"TopicArn"`
	RequestID string `json:"RequestId"`
}

type TopicEntry struct {
	TopicArn string `json:"TopicArn"`
}
