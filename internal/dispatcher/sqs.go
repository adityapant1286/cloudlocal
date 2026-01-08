package dispatcher

import (
	"bytes"
	"cloudlocal/internal/utils"
	"fmt"
	"io"
	"net/http"
)

func (d *Dispatcher) HandleSQSAdmin(w http.ResponseWriter, r *http.Request) {
	action := ""
	var payload []byte

	switch r.URL.Path {
	case "/dashboard/api/sqs/list":
		action = "ListQueues"
		payload = []byte("{}")
	case "/dashboard/api/sqs/attributes":
		url := r.URL.Query().Get("url")
		action = "GetQueueAttributes"
		payload = []byte(fmt.Sprintf(`{"QueueUrl": "%s", "AttributeNames": ["All"]}`, url))
	case "/dashboard/api/sqs/receive":
		url := r.URL.Query().Get("url")
		action = "ReceiveMessage"
		payload = []byte(fmt.Sprintf(`{"QueueUrl": "%s", "MaxNumberOfMessages": 10, "WaitTimeSeconds": 0}`, url))
	case "/dashboard/api/sqs/send":
		action = "SendMessage"
		body, _ := io.ReadAll(r.Body)
		payload = body
	case "/dashboard/api/sqs/delete-message":
		url := r.URL.Query().Get("url")
		handle := r.URL.Query().Get("handle")
		action = "DeleteMessage"
		payload = []byte(fmt.Sprintf(`{"QueueUrl": "%s", "ReceiptHandle": "%s"}`, url, handle))
	case "/dashboard/api/sqs/purge":
		url := r.URL.Query().Get("url")
		action = "PurgeQueue"
		payload = []byte(fmt.Sprintf(`{"QueueUrl": "%s"}`, url))
	}

	if action != "" {
		d.ProxyToSQS(w, action, payload)
	}
}

func (d *Dispatcher) ProxyToSQS(w http.ResponseWriter, action string, payload []byte) {
	req, _ := http.NewRequest("POST", utils.CloudLocalUrl, bytes.NewBuffer(payload))

	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AmazonSQS."+action)
	req.Header.Set("x-amz-date", "20260101T000000Z")
	req.Header.Set("Authorization", utils.ApiAuthHeader("sqs"))

	// Auth header...
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "SQS Proxy Error: "+err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	// Forward the DynamoDB response back to the Dashboard
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "SQS Proxy Copy Error: "+err.Error(), 500)
		return
	}
}
