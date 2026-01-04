package dispatcher

import (
	"bytes"
	"cloudlocal/internal/utils"
	"fmt"
	"io"
	"net/http"
)

func (d *Dispatcher) HandleS3Admin(w http.ResponseWriter, r *http.Request) {
	action := ""
	var payload []byte

	switch r.URL.Path {
	case "/dashboard/api/s3/list-buckets":
		action = "ListBuckets"
		payload = []byte("{}")
	case "/dashboard/api/s3/list-objects":
		bucket := r.URL.Query().Get("bucket")
		action = "ListObjectsV2"
		payload = []byte(fmt.Sprintf(`{"Bucket": "%s"}`, bucket))
	case "/dashboard/api/s3/upload":
		// For simplicity, we'll handle small text/JSON uploads via proxy
		action = "PutObject"
		body, _ := io.ReadAll(r.Body)
		payload = body
	case "/dashboard/api/s3/delete":
		bucket := r.URL.Query().Get("bucket")
		key := r.URL.Query().Get("key")
		action = "DeleteObject"
		payload = []byte(fmt.Sprintf(`{"Bucket": "%s", "Key": "%s"}`, bucket, key))
	case "/dashboard/api/s3/get-object":
		bucket := r.URL.Query().Get("bucket")
		key := r.URL.Query().Get("key")
		action = "GetObject"
		// GetObject typically doesn't need a JSON body in the proxy,
		// but some proxies prefer it for consistency.
		payload = []byte(fmt.Sprintf(`{"Bucket": "%s", "Key": "%s"}`, bucket, key))
	}

	if action != "" {
		d.ProxyToS3(w, r, action, payload)
	}
}

func (d *Dispatcher) ProxyToS3(w http.ResponseWriter, r *http.Request, action string, payload []byte) {
	req, _ := http.NewRequest("POST", utils.CloudLocalUrl, bytes.NewBuffer(payload))
	req.Header.Set("X-Amz-Target", "AmazonS3."+action)
	// S3 often uses XML, but LocalStack's JSON proxy supports this header:
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("x-amz-date", "20260101T000000Z")
	req.Header.Set("Authorization", utils.ApiAuthHeader("s3"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "S3 Proxy Error: "+err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	// Forward the DynamoDB response back to the Dashboard
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "S3 Proxy Copy Error: "+err.Error(), 500)
		return
	}
}
