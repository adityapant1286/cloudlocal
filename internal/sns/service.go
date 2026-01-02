package sns

import (
	"cloudlocal/internal/sqs"
	"cloudlocal/internal/utils"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

type ServiceHandler interface {
	Handle(w http.ResponseWriter, r *http.Request, body url.Values)
}

func NewSnsService(sqsSvc sqs.ServiceHandler) ServiceHandler {
	var snsEnabled = utils.IsServiceEnabled("sns")

	if snsEnabled {
		return &snsServiceImplementation{
			sns: newSns(sqsSvc),
		}
	}
	return nil
}

func (svc *snsServiceImplementation) Handle(w http.ResponseWriter, r *http.Request, body url.Values) {
	action := body.Get("Action")

	switch action {
	case "CreateTopic":
		topicName := body.Get("Name")
		result := svc.sns.createTopic(topicName)
		utils.RespondJSON(w, map[string]any{
			"ResponseMetadata": map[string]any{
				"RequestId": result.RequestID,
			},
			"TopicArn": result.TopicArn,
		})
		w.WriteHeader(200)

	case "Publish":
		topicArn := body.Get("TopicArn")
		message := body.Get("Message")
		msgId := svc.sns.publish(topicArn, message)
		utils.RespondJSON(w, map[string]any{
			"MessageId": msgId,
		})
		w.WriteHeader(200)

	case "Subscribe":
		topicArn := body.Get("TopicArn")
		endpoint := body.Get("Endpoint")
		arn := svc.sns.subscribe(topicArn, endpoint)
		utils.RespondJSON(w, map[string]any{
			"SubscriptionArn": arn,
		})
		w.WriteHeader(200)

	case "ListTopics":
		topics := svc.sns.listTopics()
		utils.RespondJSON(w, map[string]any{
			"Topics": topics,
		})
		w.WriteHeader(200)
	}
}

type internalSns interface {
	createTopic(name string) CreateTopicResponse
	subscribe(topicArn string, endpoint string) string
	publish(topicArn string, message string) string
	listTopics() []TopicEntry
}

func newSns(sqsSvc sqs.ServiceHandler) internalSns {

	return &snsImplementation{
		topics:        make(map[string]bool),
		subscriptions: make(map[string][]string),
		sqsSvc:        sqsSvc,
	}
}

func (s *snsImplementation) createTopic(name string) CreateTopicResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	arn := fmt.Sprintf("arn:aws:sns:%s:%s:%s", utils.AwsRegion, utils.AccountId, name)
	s.topics[arn] = true
	return CreateTopicResponse{
		TopicArn:  arn,
		RequestID: utils.RandomUuid(),
	}
}

func (s *snsImplementation) subscribe(topicArn string, endpoint string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscriptions[topicArn] = append(s.subscriptions[topicArn], endpoint)
	epochMillis := time.Now().UnixMilli()
	return fmt.Sprintf("arn:aws:sns:%s:%s:%s:sub-%d", utils.AwsRegion, utils.AccountId, topicArn, epochMillis)
}

func (s *snsImplementation) publish(topicArn string, message string) string {
	s.mu.RLock()
	endpoints := s.subscriptions[topicArn]
	s.mu.RUnlock()

	for _, queueUrl := range endpoints {
		// Directly inject into our SQS service
		_, err := s.sqsSvc.SendMessage(queueUrl, message)
		if err != nil {
			log.Fatalf("Error publishing to SQS queue %s: %s", queueUrl, err.Error())
		}
	}
	return utils.RandomUuid()
}

func (s *snsImplementation) listTopics() []TopicEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var topicArns []TopicEntry
	for arn := range s.topics {
		topicArns = append(topicArns, TopicEntry{
			TopicArn: arn,
		})
	}
	return topicArns
}
