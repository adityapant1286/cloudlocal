package dispatcher

import (
	"bytes"
	"cloudlocal/internal/utils"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
)

type proxyS3Input struct {
	writer http.ResponseWriter
	method string
	url    string
	action string
	reader io.Reader
}

func (d *Dispatcher) HandleS3Admin(w http.ResponseWriter, r *http.Request) {
	//action := ""
	//method := http.MethodPost
	//var payload []byte
	var input *proxyS3Input = nil

	switch r.URL.Path {
	case "/dashboard/api/s3/list-buckets":
		input = &proxyS3Input{
			writer: w,
			method: http.MethodGet,
			url:    utils.CloudLocalUrl + "/",
			action: "ListBuckets",
		}
	case "/dashboard/api/s3/list-objects":
		bucket := r.URL.Query().Get("bucket")
		input = &proxyS3Input{
			writer: w,
			method: http.MethodGet,
			url:    utils.CloudLocalUrl + "/" + bucket + "?list-type=2",
			action: "ListObjectsV2",
		}
	case "/dashboard/api/s3/create-bucket":
		bucket := r.URL.Query().Get("name")
		input = &proxyS3Input{
			writer: w,
			method: http.MethodPut,
			url:    utils.CloudLocalUrl + "/" + bucket + "/",
			action: "CreateBucket",
		}
	case "/dashboard/api/s3/delete-bucket":
		bucket := r.URL.Query().Get("name")
		input = &proxyS3Input{
			writer: w,
			method: http.MethodDelete,
			url:    utils.CloudLocalUrl + "/" + bucket + "/",
			action: "DeleteBucket",
		}
	case "/dashboard/api/s3/upload":
		var s3UploadPayload struct {
			Bucket string `json:"Bucket"`
			Key    string `json:"Key"`
			Body   string `json:"Body"` // Base64 from frontend
		}
		_ = json.NewDecoder(r.Body).Decode(&s3UploadPayload)
		rawBody, _ := base64.StdEncoding.DecodeString(s3UploadPayload.Body)
		input = &proxyS3Input{
			writer: w,
			method: http.MethodPut,
			url:    utils.CloudLocalUrl + "/" + s3UploadPayload.Bucket + "/" + s3UploadPayload.Key,
			action: "PutObject",
			reader: bytes.NewReader(rawBody),
		}
	case "/dashboard/api/s3/get-object":
		bucket := r.URL.Query().Get("bucket")
		key := r.URL.Query().Get("key")
		input = &proxyS3Input{
			writer: w,
			method: http.MethodGet,
			url:    utils.CloudLocalUrl + "/" + bucket + "/" + key,
			action: "GetObject",
		}
	case "/dashboard/api/s3/delete":
		bucket := r.URL.Query().Get("bucket")
		key := r.URL.Query().Get("key")
		input = &proxyS3Input{
			writer: w,
			method: http.MethodDelete,
			url:    utils.CloudLocalUrl + "/" + bucket + "/" + key,
			action: "DeleteObject",
		}
	}

	if input != nil {
		d.ProxyToS3(*input)
	}
}

func (d *Dispatcher) ProxyToS3(input proxyS3Input) {

	req, _ := http.NewRequest(input.method, input.url, input.reader)
	//req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AmazonS3."+input.action)
	req.Header.Set("x-amz-date", utils.XAmzDate())
	req.Header.Set("Authorization", utils.ApiAuthHeader("s3"))

	writer := input.writer

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(writer, "S3 Proxy Error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Forward the DynamoDB response back to the Dashboard

	if input.action == "GetObject" {
		writer.Header().Set("Content-Type", "application/json")
		data, _ := io.ReadAll(resp.Body)
		utils.RespondJSON(writer, map[string]string{"Body": base64.StdEncoding.EncodeToString(data)})
		return
	}
	writer.WriteHeader(resp.StatusCode)
	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		http.Error(writer, "S3 Proxy Copy Error: "+err.Error(), 500)
		return
	}
}
