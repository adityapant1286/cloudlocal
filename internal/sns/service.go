package sns

import (
	"cloudlocal/internal/cloudwatch"
	"cloudlocal/internal/sqs"
	"cloudlocal/internal/utils"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type ServiceHandler interface {
	Handle(w http.ResponseWriter, body url.Values)
}

func NewSnsService(sqsSvc sqs.ServiceHandler) ServiceHandler {
	cw := cloudwatch.GetServiceInstance()
	var snsEnabled = utils.IsServiceEnabled(SNS)

	if snsEnabled {
		return &snsServiceImplementation{
			sns: newSns(cw, sqsSvc),
		}
	}
	return nil
}

func (svc *snsServiceImplementation) Handle(w http.ResponseWriter, body url.Values) {
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
		w.WriteHeader(http.StatusOK)

	case "Publish":
		topicArn := body.Get("TopicArn")
		message := body.Get("Message")
		msgId := svc.sns.publish(topicArn, message)
		utils.RespondJSON(w, map[string]any{
			"MessageId": msgId,
		})
		w.WriteHeader(http.StatusOK)

	case "Subscribe":
		topicArn := body.Get("TopicArn")
		endpoint := body.Get("Endpoint")
		arn := svc.sns.subscribe(topicArn, endpoint)
		utils.RespondJSON(w, map[string]any{
			"SubscriptionArn": arn,
		})
		w.WriteHeader(http.StatusOK)

	case "ListTopics":
		topics := svc.sns.listTopics()
		utils.RespondJSON(w, map[string]any{
			"Topics": topics,
		})
		w.WriteHeader(http.StatusOK)
	}
}

type internalSns interface {
	createTopic(name string) CreateTopicResponse
	subscribe(topicArn string, endpoint string) string
	publish(topicArn string, message string) string
	listTopics() []TopicEntry
}

func newSns(cw cloudwatch.CwService, sqsSvc sqs.ServiceHandler) internalSns {

	return &snsImplementation{
		cloudwatch:    cw,
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
	resp := CreateTopicResponse{
		TopicArn:  arn,
		RequestID: utils.RandomUuid(),
	}
	s.cloudwatch.Info(SERVICE, "CreateTopic", fmt.Sprintf("SNS Topic created. Name: %s", utils.MarshalIjson(resp)))
	return resp
}

func (s *snsImplementation) subscribe(topicArn string, endpoint string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscriptions[topicArn] = append(s.subscriptions[topicArn], endpoint)
	epochMillis := time.Now().UnixMilli()
	arn := fmt.Sprintf("arn:aws:sns:%s:%s:%s:sub-%d", utils.AwsRegion, utils.AccountId, topicArn, epochMillis)

	s.cloudwatch.Info(SERVICE, "Subscribe", fmt.Sprintf("SNS subscribe Topic ARN: %s, endpoint: %s, subscription ARN: %s", topicArn, endpoint, arn))
	return arn
}

func (s *snsImplementation) publish(topicArn string, message string) string {
	s.mu.RLock()
	endpoints := s.subscriptions[topicArn]
	s.mu.RUnlock()

	for _, queueUrl := range endpoints {
		// Directly inject into our SQS service
		_, err := s.sqsSvc.SendMessage(queueUrl, message)
		if err != nil {
			s.cloudwatch.Error(SERVICE, "Publish", fmt.Sprintf("SNS publish error. Topic ARN: %s, endpoint: %s, error: %s", topicArn, queueUrl, err.Error()))
		}
	}

	uuid := utils.RandomUuid()
	s.cloudwatch.Info(SERVICE, "Publish", fmt.Sprintf("SNS publish complete. Topic ARN: %s, publish id: %s", topicArn, uuid))
	return uuid
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
	s.cloudwatch.Info(SERVICE, "ListTopics", fmt.Sprintf("SNS Listing topics Topic ARNs: %s", utils.MarshalIjson(topicArns)))
	return topicArns
}
