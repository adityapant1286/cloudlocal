package dispatcher

import (
	"bytes"
	"io"
	"net/http"
)

func (d *Dispatcher) HandleKMSAdmin(w http.ResponseWriter, r *http.Request) {
	action := ""
	var payload []byte

	switch r.URL.Path {
	case "/dashboard/api/kms/list":
		action = "ListKeys"
		payload = []byte("{}")
	case "/dashboard/api/kms/create":
		action = "CreateKey"
		payload = []byte(`{"Description": "Created via CloudLocal Dashboard"}`)
	case "/dashboard/api/kms/encrypt":
		action = "Encrypt"
		body, _ := io.ReadAll(r.Body)
		payload = body
	case "/dashboard/api/kms/decrypt":
		action = "Decrypt"
		body, _ := io.ReadAll(r.Body)
		payload = body
	}

	if action != "" {
		d.ProxyToKMS(w, r, action, payload)
	}
}

func (d *Dispatcher) ProxyToKMS(w http.ResponseWriter, r *http.Request, action string, payload []byte) {
	url := "http://localhost:10050"
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))

	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "TrentService."+action) // KMS uses "TrentService"

	// Hardcoded dummy auth as established
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=cloudlocal/20260104/ap-southeast-2/kms/aws4_request, ...")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "KMS Proxy Error: "+err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "KMS Proxy Copy Error: "+err.Error(), 500)
		return
	}
}
