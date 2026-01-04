package dispatcher

import (
	"bytes"
	"cloudlocal/internal/utils"
	"fmt"
	"io"
	"net/http"
)

func (d *Dispatcher) HandleSecretsAdmin(w http.ResponseWriter, r *http.Request) {
	action := ""
	var payload []byte

	switch r.URL.Path {
	case "/dashboard/api/secrets/list":
		action = "ListSecrets"
		payload = []byte("{}")
	case "/dashboard/api/secrets/get":
		name := r.URL.Query().Get("name")
		action = "GetSecretValue"
		payload = []byte(fmt.Sprintf(`{"SecretId": "%s"}`, name))
	case "/dashboard/api/secrets/create":
		action = "CreateSecret"
		body, _ := io.ReadAll(r.Body)
		payload = body
	case "/dashboard/api/secrets/put":
		action = "PutSecretValue"
		body, _ := io.ReadAll(r.Body)
		payload = body
	case "/dashboard/api/secrets/delete":
		name := r.URL.Query().Get("name")
		action = "DeleteSecret"
		// RecoveryWindowInDays: 0 forces immediate deletion in LocalStack/CloudLocal
		payload = []byte(fmt.Sprintf(`{"SecretId": "%s", "ForceDeleteWithoutRecovery": true}`, name))
	}

	if action != "" {
		// Use your existing ProxyToDynamo but change the Target Header prefix
		d.ProxyToSecrets(w, r, action, payload)
	}
}

// ProxyToSecrets is identical to ProxyToDynamo but with a different Target header
func (d *Dispatcher) ProxyToSecrets(w http.ResponseWriter, r *http.Request, action string, payload []byte) {
	req, _ := http.NewRequest("POST", utils.CloudLocalUrl, bytes.NewBuffer(payload))

	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "secretsmanager."+action)
	req.Header.Set("x-amz-date", "20260101T000000Z")
	req.Header.Set("Authorization", utils.ApiAuthHeader("secretsmanager"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Secrets Proxy Error: "+err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	// Forward the DynamoDB response back to the Dashboard
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "Secrets Proxy Copy Error: "+err.Error(), 500)
		return
	}
}
