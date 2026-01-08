package sqs

import (
	"cloudlocal/internal/cloudwatch"
	"sync"
	"time"
)

const SQS = "sqs"
const SERVICE = SQS + "-service"

type sqsImplementation struct {
	cloudwatch cloudwatch.CwService
	sqs        internalSqsService
}

type sqsQueueImplementation struct {
	mu         sync.RWMutex
	cloudwatch cloudwatch.CwService
	//Arn    string
	queues map[string]*Queue
}

type RedrivePolicy struct {
	DeadLetterTargetArn string `json:"deadLetterTargetArn"`
	MaxReceiveCount     int    `json:"maxReceiveCount"`
}

type Message struct {
	MessageId     string            `json:"MessageId"`
	Body          string            `json:"Body"`
	ReceiptHandle string            `json:"ReceiptHandle"` // Used for deleting
	MD5OfBody     string            `json:"MD5OfBody"`
	Attributes    map[string]string `json:"Attributes"`
	VisibleAt     time.Time         `json:"-"` // Not exported to JSON
	ReceiveCount  int               `json:"-"` // Track attempts
}

type Queue struct {
	URL           string
	ARN           string
	Messages      []Message
	RedrivePolicy *RedrivePolicy
}
