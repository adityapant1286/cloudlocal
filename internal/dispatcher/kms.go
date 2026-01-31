package dispatcher

import (
	"bytes"
	"cloudlocal/internal/utils"
	"fmt"
	"io"
	"log"
	"net/http"
)

func (d *Dispatcher) HandleKMSAdmin(w http.ResponseWriter, r *http.Request) {
	action := ""
	var payload []byte

	log.Printf("HandleKMSAdmin: %v", r.URL)

	switch r.URL.Path {
	case "/dashboard/api/kms/list":
		action = "ListKeys"
		payload = []byte("{}")
	case "/dashboard/api/kms/create-alias":
		keyId := r.URL.Query().Get("keyId")
		name := r.URL.Query().Get("name")
		action = "CreateAlias"
		payload = []byte(fmt.Sprintf(`{"TargetKeyId": "%s", "AliasName": "%s"}`, keyId, name))
	case "/dashboard/api/kms/list-aliases":
		keyId := r.URL.Query().Get("keyId")
		action = "ListAliases"
		payload = []byte(fmt.Sprintf(`{"KeyId": "%s"}`, keyId))
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
	req.Header.Set("x-amz-date", utils.XAmzDate())
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
