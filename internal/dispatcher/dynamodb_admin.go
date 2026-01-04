package dispatcher

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

func (d *Dispatcher) HandleDynamoAdmin(w http.ResponseWriter, r *http.Request) {
	action := ""
	var payload []byte

	switch r.URL.Path {
	case "/dashboard/api/dynamo/tables":
		action = "ListTables"
		payload = []byte("{}")
	case "/dashboard/api/dynamo/scan":
		action = "Scan"
		body, _ := io.ReadAll(r.Body)
		payload = body
	case "/dashboard/api/dynamo/describe":
		action = "DescribeTable"
		tableName := r.URL.Query().Get("table")
		payload = []byte(fmt.Sprintf(`{"TableName": "%s"}`, tableName))
	case "/dashboard/api/dynamo/put_item":
		action = "PutItem"
		body, _ := io.ReadAll(r.Body)
		payload = body
	case "/dashboard/api/dynamo/delete":
		action = "DeleteItem"
		body, _ := io.ReadAll(r.Body)
		payload = body
	}

	if action != "" {
		d.ProxyToDynamo(w, r, action, payload)
	}
}

// ProxyToDynamo forwards dashboard requests to the local DynamoDB process
func (d *Dispatcher) ProxyToDynamo(w http.ResponseWriter, r *http.Request, action string, payload []byte) {
	// DynamoDB Local is on 10051
	url := "http://localhost:10051"

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))

	// Crucial: DynamoDB Local requires these specific headers
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "DynamoDB_20120810."+action)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=cloudlocal/20250101/ap-southeast-2/dynamodb/aws4_request, SignedHeaders=host;x-amz-date;x-amz-target, Signature=dummy")
	req.Header.Set("x-amz-date", "20250101T000000Z")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "DynamoDB Proxy Error: "+err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	// Forward the DynamoDB response back to the Dashboard
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "DynamoDB Proxy Copy Error: "+err.Error(), 500)
		return
	}
}
