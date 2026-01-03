package dispatcher

import (
	"bytes"
	"io"
	"net/http"
)

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
