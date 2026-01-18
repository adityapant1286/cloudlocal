package sqs

import (
	"cloudlocal/internal/cloudwatch"
	"cloudlocal/internal/utils"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	cw := cloudwatch.GetServiceInstance()
	var sqsEnabled = utils.IsServiceEnabled(SQS)

	if sqsEnabled {
		return &sqsImplementation{
			cloudwatch: cw,
			sqs:        newSqsService(cw),
		}
	}
	return nil
}

func (svc *sqsImplementation) Handle(w http.ResponseWriter, r *http.Request, target string) {
	err := r.ParseForm()
	if err != nil {
		svc.cloudwatch.Error(SERVICE, "SQSHttpHandler", fmt.Sprintf("Error parsing form: %s|%s", target, err.Error()))
		return
	}

	var action = r.FormValue("Action")

	if action == "" {
		action = strings.ReplaceAll(target, "AmazonSQS.", "")
	}

	switch action {
	case "ListQueues":
		queues := svc.sqs.listQueues()
		utils.RespondJSON(w, map[string][]Queue{"Queues": queues})
	case "CreateQueue":
		name := r.FormValue("QueueName")

		if name == "" {
			body, _ := io.ReadAll(r.Body)
			var req struct {
				QueueName string `json:"QueueName"`
			}
			utils.UnmarshalJson(body, &req)
			name = req.QueueName
		}

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
		utils.RespondJSON(w, q)
	case "SendMessage":
		url := r.FormValue("QueueUrl")
		body := r.FormValue("MessageBody")

		if url == "" {
			rbody, _ := io.ReadAll(r.Body)
			var req struct {
				QueueUrl    string `json:"QueueUrl"`
				MessageBody string `json:"MessageBody"`
			}
			utils.UnmarshalJson(rbody, &req)
			url = req.QueueUrl
			body = req.MessageBody
		}
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
		maxMessages := utils.ToInt(r.FormValue("MaxNumberOfMessages"))
		if url == "" {
			rbody, _ := io.ReadAll(r.Body)

			var req struct {
				QueueUrl            string `json:"QueueUrl"`
				MaxNumberOfMessages int    `json:"MaxNumberOfMessages"`
				WaitTimeSeconds     int    `json:"WaitTimeSeconds"`
			}
			utils.UnmarshalJson(rbody, &req)
			url = req.QueueUrl
			maxMessages = req.MaxNumberOfMessages
		}
		msgs, err := svc.sqs.receiveMessage(url, maxMessages)
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
		receiptHandle := r.FormValue("ReceiptHandle")
		if url == "" {
			rbody, _ := io.ReadAll(r.Body)

			var req struct {
				QueueUrl      string `json:"QueueUrl"`
				ReceiptHandle string `json:"ReceiptHandle"`
			}
			utils.UnmarshalJson(rbody, &req)
			url = req.QueueUrl
			receiptHandle = req.ReceiptHandle
		}

		err := svc.sqs.deleteMessage(url, receiptHandle)
		if err != nil {
			utils.RespondError(utils.RespInput{
				Writer:   w,
				Code:     http.StatusUnprocessableEntity,
				ErrorStr: "DeleteMessageException",
				Data:     map[string]string{"message": err.Error()},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	case "PurgeQueue":
		url := r.FormValue("QueueUrl")
		if url == "" {
			rbody, _ := io.ReadAll(r.Body)

			var req struct {
				QueueUrl string `json:"QueueUrl"`
			}
			utils.UnmarshalJson(rbody, &req)
			url = req.QueueUrl
		}
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
		w.WriteHeader(http.StatusOK)
	case "SetQueueAttributes":
		url := r.FormValue("QueueUrl")
		// AWS sends attributes as Attribute.1.Name=RedrivePolicy & Attribute.1.Value={...}
		policyJson := r.FormValue("Attribute.1.Value")

		var policy RedrivePolicy
		utils.UnmarshalJson([]byte(policyJson), &policy)
		svc.sqs.setRedrivePolicy(url, &policy)

		w.WriteHeader(http.StatusOK)
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
	listQueues() []Queue
	purgeQueue(queueURL string) error
	setRedrivePolicy(queueURL string, policy *RedrivePolicy)
}

func newSqsService(cw cloudwatch.CwService) internalSqsService {
	return &sqsQueueImplementation{
		cloudwatch: cw,
		queues:     make(map[string]*Queue),
	}
}

func (s *sqsQueueImplementation) listQueues() []Queue {
	s.mu.Lock()
	defer s.mu.Unlock()

	queueList := make([]Queue, 0, len(s.queues))
	for _, v := range s.queues {
		queueList = append(queueList, *v)
	}

	return queueList
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

	s.cloudwatch.Info(SERVICE, "CreateQueue", fmt.Sprintf("Created SQS Queue: %s", utils.MarshalIjson(q)))

	return q, nil
}

func (s *sqsQueueImplementation) sendMessage(queueURL, body string) (*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	queue, ok := s.queues[queueURL]
	if !ok {
		return nil, fmt.Errorf("QueueNotFound")
	}

	md5 := utils.HashMd5([]byte(body))
	msgId := fmt.Sprintf("msg-%d", time.Now().UnixNano())
	msg := &Message{
		MessageId:     msgId,
		Body:          body,
		ReceiptHandle: msgId + "-handle",
		MD5OfBody:     md5,
	}

	queue.Messages = append(queue.Messages, *msg)
	s.queues[queueURL] = queue
	s.cloudwatch.Info(SERVICE, "SendMessage", fmt.Sprintf("SQS Message: %s", utils.MarshalIjson(msg)))

	return msg, nil
}

func (s *sqsQueueImplementation) receiveMessage(queueURL string, maxMessages int) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queue, ok := s.queues[queueURL]

	if !ok || queue == nil {
		return nil, errors.New("QueueNotFound")
	}

	msgCount := len(queue.Messages)

	if maxMessages > msgCount {
		maxMessages = msgCount
	}

	var result []Message
	var redrivePolicyMaxCount = -1
	var deadLetterQueuePresent = false
	if queue.RedrivePolicy != nil {

		deadLetterQueuePresent = queue.RedrivePolicy.DeadLetterTargetArn != ""

		if queue.RedrivePolicy.MaxReceiveCount > 0 {
			redrivePolicyMaxCount = queue.RedrivePolicy.MaxReceiveCount
		}
	}
	now := time.Now()

	for i := 0; i < maxMessages; i++ {
		msg := queue.Messages[i]

		if isBefore(msg.VisibleAt, now) {
			msg.ReceiveCount++

			if isMoveDeadLetterQueue(msg, deadLetterQueuePresent, redrivePolicyMaxCount) {
				s.moveToDLQ(queueURL, msg)
				queue.Messages = append(queue.Messages[:i], queue.Messages[i+1:]...)
				i--
				continue
			}

			msg.VisibleAt = now.Add(30 * time.Second)
			result = append(result, msg)
		}
	}
	s.queues[queueURL] = queue
	s.cloudwatch.Info(SERVICE, "ReceiveMessage", fmt.Sprintf("SQS Messages: %s", utils.MarshalIjson(map[string]any{
		"MaxNumberOfMessages": maxMessages,
		"QueueUrl":            queueURL,
		"Messages":            result,
	})))

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
	var msgs []Message
	for _, m := range queue.Messages {
		if m.ReceiptHandle != receiptHandle {
			msgs = append(msgs, m)
		}
	}
	s.queues[queueURL].Messages = msgs
	s.cloudwatch.Info(SERVICE, "DeleteMessage", fmt.Sprintf("Delete SQS Message: %s", utils.MarshalIjson(map[string]any{
		"QueueUrl":                  queueURL,
		"OldNumberOfMessages":       beforeDelete,
		"RemainingNumberOfMessages": len(queue.Messages),
	})))
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
	s.queues[queueURL].Messages = []Message{}
	s.cloudwatch.Info(SERVICE, "PurgeQueue", fmt.Sprintf("Purged SQS Queue: %s", utils.MarshalIjson(map[string]any{
		"QueueUrl":               queueURL,
		"NumberOfMessagesPurged": noOfMsgs,
	})))

	return nil
}

func (s *sqsQueueImplementation) setRedrivePolicy(queueURL string, policy *RedrivePolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()

	queue, ok := s.queues[queueURL]
	if !ok {
		s.cloudwatch.Error(SERVICE, "SetRedrivePolicy", fmt.Sprintf("Queue not found for setting RedrivePolicy"))
	}

	queue.RedrivePolicy = policy
	s.queues[queueURL] = queue
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

func isMoveDeadLetterQueue(msg Message, deadLetterQueuePresent bool, redrivePolicyMaxCount int) bool {
	return deadLetterQueuePresent && msg.ReceiveCount >= redrivePolicyMaxCount
}
