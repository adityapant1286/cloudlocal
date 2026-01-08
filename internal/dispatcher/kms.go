package dispatcher

import (
	"bytes"
	"cloudlocal/internal/utils"
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
		d.ProxyToKMS(w, action, payload)
	}
}

func (d *Dispatcher) ProxyToKMS(w http.ResponseWriter, action string, payload []byte) {
	req, _ := http.NewRequest("POST", utils.CloudLocalUrl, bytes.NewBuffer(payload))

	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "TrentService."+action) // KMS uses "TrentService"
	req.Header.Set("x-amz-date", "20260101T000000Z")
	req.Header.Set("Authorization", utils.ApiAuthHeader("kms"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "KMS Proxy Error: "+err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "KMS Proxy Copy Error: "+err.Error(), 500)
		return
	}
}
