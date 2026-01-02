package sqs

import (
	"cloudlocal/internal/utils"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

type QueuePurger interface {
	FlushQueue(queueURL string) error
}

type MessageSender interface {
	SendMessage(queueURL, body string) (Message, error)
}

type ServiceHandler interface {
	utils.ServiceHandler
	QueuePurger
	MessageSender
}

func NewSQSService() ServiceHandler {
	var sqsEnabled = utils.IsServiceEnabled("sqs")

	if sqsEnabled {
		return &sqsImplementation{
			sqs: newSqsService(),
		}
	}
	return nil
}

func (svc *sqsImplementation) Handle(w http.ResponseWriter, r *http.Request, target string) {
	err := r.ParseForm()
	if err != nil {
		log.Fatalf("Error parsing form: %s|%s", target, err.Error())
		return
	}
	action := r.FormValue("Action")

	switch action {
	case "CreateQueue":
		name := r.FormValue("QueueName")
		q, err := svc.sqs.createQueue(name)
		if err != nil {
			utils.RespondError(utils.RespInput{
				Writer:   w,
				Code:     http.StatusBadRequest,
				ErrorStr: "ResourceNotCreatedException",
				Data:     map[string]string{"message": err.Error()},
			})
			return
		}
		utils.RespondJSON(w, map[string]string{"QueueUrl": q.URL})
	case "SendMessage":
		url := r.FormValue("QueueUrl")
		body := r.FormValue("MessageBody")
		msg, err := svc.sqs.sendMessage(url, body)
		if err != nil {
			utils.RespondError(utils.RespInput{
				Writer:   w,
				Code:     http.StatusUnprocessableEntity,
				ErrorStr: "SendMessageException",
				Data:     map[string]string{"message": err.Error()},
			})
			return
		}
		utils.RespondJSON(w, map[string]string{
			"MessageId":        msg.MessageId,
			"MD5OfMessageBody": msg.MD5OfBody,
		})
	case "ReceiveMessage":
		url := r.FormValue("QueueUrl")
		maxMessages := r.FormValue("MaxNumberOfMessages")
		msgs, err := svc.sqs.receiveMessage(url, utils.ToInt(maxMessages))
		if err != nil {
			utils.RespondError(utils.RespInput{
				Writer:   w,
				Code:     http.StatusUnprocessableEntity,
				ErrorStr: "ReceiveMessageException",
				Data:     map[string]string{"message": err.Error()},
			})
			return
		}
		utils.RespondJSON(w, map[string]interface{}{"Messages": msgs})
	case "DeleteMessage":
		url := r.FormValue("QueueUrl")
		handle := r.FormValue("ReceiptHandle")
		err := svc.sqs.deleteMessage(url, handle)
		if err != nil {
			utils.RespondError(utils.RespInput{
				Writer:   w,
				Code:     http.StatusUnprocessableEntity,
				ErrorStr: "DeleteMessageException",
				Data:     map[string]string{"message": err.Error()},
			})
			return
		}
		w.WriteHeader(200)
	case "PurgeQueue":
		url := r.FormValue("QueueUrl")
		err := svc.sqs.purgeQueue(url)
		if err != nil {
			utils.RespondError(utils.RespInput{
				Writer:   w,
				Code:     http.StatusBadRequest,
				ErrorStr: "QueueDoesNotExist",
				Data:     map[string]string{"message": err.Error()},
			})
			return
		}
		w.WriteHeader(200)
	case "SetQueueAttributes":
		url := r.FormValue("QueueUrl")
		// AWS sends attributes as Attribute.1.Name=RedrivePolicy & Attribute.1.Value={...}
		policyJson := r.FormValue("Attribute.1.Value")

		var policy RedrivePolicy
		utils.UnmarshalJson([]byte(policyJson), &policy)
		svc.sqs.setRedrivePolicy(url, &policy)

		w.WriteHeader(200)
	}
}

func (svc *sqsImplementation) FlushQueue(queueURL string) error {
	return svc.sqs.purgeQueue(queueURL)
}

func (svc *sqsImplementation) SendMessage(queueURL, body string) (Message, error) {
	message, err := svc.sqs.sendMessage(queueURL, body)
	return *message, err
}

type internalSqsService interface {
	createQueue(name string) (*Queue, error)
	sendMessage(queueURL, body string) (*Message, error)
	receiveMessage(queueURL string, maxMessages int) ([]Message, error)
	deleteMessage(queueURL, receiptHandle string) error
	purgeQueue(queueURL string) error
	setRedrivePolicy(queueURL string, policy *RedrivePolicy)
}

func newSqsService() internalSqsService {
	return &sqsQueueImplementation{
		queues: make(map[string]*Queue),
	}
}

func (s *sqsQueueImplementation) createQueue(name string) (*Queue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	arn := fmt.Sprintf("arn:aws:sqs:%s:%s:%s", utils.AwsRegion, utils.AccountId, name)
	url := fmt.Sprintf("%s/sqs/%s", utils.CloudLocalUrl, name)
	q := &Queue{
		URL:      url,
		ARN:      arn,
		Messages: []Message{},
	}
	s.queues[url] = q

	log.Printf("Created SQS Queue:\n%s\n\n", utils.MarshalIjson(q))

	return q, nil
}

func (s *sqsQueueImplementation) sendMessage(queueURL, body string) (*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	queue, ok := s.queues[queueURL]
	if !ok {
		return nil, fmt.Errorf("QueueNotFound")
	}

	msgId := fmt.Sprintf("msg-%d", time.Now().UnixNano())
	msg := Message{
		MessageId:     msgId,
		Body:          body,
		ReceiptHandle: msgId + "-handle",                  // Simple mock handle
		MD5OfBody:     "7b52009b64fd0a2a49e6d8a939753077", // Placeholder MD5
	}

	queue.Messages = append(queue.Messages, msg)
	log.Printf("SQS Message:\n%s\n\n", utils.MarshalIjson(msg))

	return &msg, nil
}

func (s *sqsQueueImplementation) receiveMessage(queueURL string, maxMessages int) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queue, ok := s.queues[queueURL]
	if !ok || len(queue.Messages) == 0 {
		return nil, errors.New("QueueNotFound")
	}

	if maxMessages > len(queue.Messages) {
		maxMessages = len(queue.Messages)
	}

	var result []Message
	var redrivePolicyMaxCount = -1
	if queue.RedrivePolicy != nil && queue.RedrivePolicy.MaxReceiveCount > 0 {
		redrivePolicyMaxCount = queue.RedrivePolicy.MaxReceiveCount
	}
	now := time.Now()

	for i := 0; i < maxMessages; i++ {

		msg := &queue.Messages[i]

		if isBefore(msg.VisibleAt, now) {
			msg.ReceiveCount++

			if isMoveDeadLetterQueue(msg, redrivePolicyMaxCount) {
				s.moveToDLQ(queueURL, *msg)
				queue.Messages = append(queue.Messages[:i], queue.Messages[i+1:]...)
				i--
				continue
			}

			msg.VisibleAt = now.Add(30 * time.Second)
			result = append(result, *msg)
		}
	}
	log.Printf("SQS Messages:\n%s\n\n", utils.MarshalIjson(map[string]any{
		"MaxNumberOfMessages": maxMessages,
		"QueueUrl":            queueURL,
		"Messages":            result,
	}))

	return result, nil
}

func (s *sqsQueueImplementation) deleteMessage(queueURL, receiptHandle string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	queue, ok := s.queues[queueURL]
	if !ok {
		return errors.New("QueueNotFound")
	}

	// Filter out the message with the matching handle
	beforeDelete := len(queue.Messages)
	var newMsgs []Message
	for _, m := range queue.Messages {
		if m.ReceiptHandle != receiptHandle {
			newMsgs = append(newMsgs, m)
		}
	}
	queue.Messages = newMsgs
	log.Printf("SQS Message:\n%s\n\n", utils.MarshalIjson(map[string]any{
		"QueueUrl":                  queueURL,
		"OldNumberOfMessages":       beforeDelete,
		"RemainingNumberOfMessages": len(queue.Messages),
	}))
	return nil
}

func (s *sqsQueueImplementation) purgeQueue(queueURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	queue, ok := s.queues[queueURL]
	if !ok {
		return errors.New("QueueDoesNotExist")
	}

	noOfMsgs := len(queue.Messages)

	// Efficiently clear the slice
	queue.Messages = []Message{}
	log.Printf("Purged SQS Queue:\n%s\n\n", utils.MarshalIjson(map[string]any{
		"QueueUrl":               queueURL,
		"NumberOfMessagesPurged": noOfMsgs,
	}))

	return nil
}

func (s *sqsQueueImplementation) setRedrivePolicy(queueURL string, policy *RedrivePolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()

	queue, ok := s.queues[queueURL]
	if !ok {
		log.Fatal("Queue not found for setting RedrivePolicy")
	}

	queue.RedrivePolicy = policy
}

func (s *sqsQueueImplementation) moveToDLQ(sourceUrl string, msg Message) {
	sourceQueue := s.queues[sourceUrl]
	targetArn := sourceQueue.RedrivePolicy.DeadLetterTargetArn

	// Find the queue that matches this ARN
	for _, q := range s.queues {
		if q.ARN == targetArn {
			msg.ReceiveCount = 0 // Reset count for the new queue
			q.Messages = append(q.Messages, msg)
			return
		}
	}
}

func isBefore(msgTime time.Time, otherTime time.Time) bool {
	return msgTime.Before(otherTime)
}

func isMoveDeadLetterQueue(msg *Message, redrivePolicyMaxCount int) bool {
	return msg.ReceiveCount >= redrivePolicyMaxCount
}
