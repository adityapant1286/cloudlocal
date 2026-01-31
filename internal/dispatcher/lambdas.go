package dispatcher

import (
	"bytes"
	"cloudlocal/internal/utils"
	"fmt"
	"io"
	"net/http"
)

func (d *Dispatcher) HandleLambdasAdmin(w http.ResponseWriter, r *http.Request) {
	action := ""
	var payload []byte

	switch r.URL.Path {
	case "/dashboard/api/lambda/list":
		action = "ListLambda"
		payload = []byte("{}")
	case "/dashboard/api/lambda/describe":
		name := r.URL.Query().Get("name")
		action = "DescribeLambda"
		payload = []byte(fmt.Sprintf(`{"functionName": "%s"}`, name))
		//case "/dashboard/api/lambda/create":
		//	action = "CreateSecret"
		//	body, _ := io.ReadAll(r.Body)
		//	payload = body
		//case "/dashboard/api/lambda/put":
		//	action = "PutSecretValue"
		//	body, _ := io.ReadAll(r.Body)
		//	payload = body
		//case "/dashboard/api/lambda/delete":
		//	name := r.URL.Query().Get("name")
		//	action = "DeleteSecret"
		//	// RecoveryWindowInDays: 0 forces immediate deletion in LocalStack/CloudLocal
		//	payload = []byte(fmt.Sprintf(`{"SecretId": "%s", "ForceDeleteWithoutRecovery": true}`, name))
	}

	if action != "" {
		// Use your existing ProxyToDynamo but change the Target Header prefix
		d.ProxyToSecrets(w, action, payload)
	}
}

func (d *Dispatcher) ProxyToLambdas(w http.ResponseWriter, action string, payload []byte) {
	req, _ := http.NewRequest("POST", utils.CloudLocalUrl, bytes.NewBuffer(payload))

	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "lambda."+action)
	req.Header.Set("x-amz-date", utils.XAmzDate())
	req.Header.Set("Authorization", utils.ApiAuthHeader("lambda"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Lambda Proxy Error: "+err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	// Forward the DynamoDB response back to the Dashboard
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "Lambda Proxy Copy Error: "+err.Error(), 500)
		return
	}
}
