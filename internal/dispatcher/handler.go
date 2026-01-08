package dispatcher

import (
	"cloudlocal/internal/health"
	"cloudlocal/internal/utils"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

func (d *Dispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" && r.URL.Path == "/dashboard" && utils.DashboardEnabled {
		http.Redirect(w, r, "/dashboard/", http.StatusFound)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/dashboard") {
		if !utils.DashboardEnabled {
			http.Error(w, "Dashboard disabled", http.StatusNotFound)
			return
		}

		// Handle API calls specifically
		if strings.HasPrefix(r.URL.Path, "/dashboard/api") {
			switch r.URL.Path {
			case "/dashboard/api/status":
				d.HandleDashboardAPI(w, r)
			case "/dashboard/api/logs":
				d.HandleLogStream(w, r)
			case "/dashboard/api/logs/download":
				d.HandleLogDownload(w, r)
			case "/dashboard/api/action":
				d.HandleDashboardAPI(w, r) // SQS Flush/S3 Clear
			}

			if strings.HasPrefix(r.URL.Path, "/dashboard/api/dynamo") {
				d.HandleDynamoAdmin(w, r)
				return
			}

			if strings.HasPrefix(r.URL.Path, "/dashboard/api/secrets") {
				d.HandleSecretsAdmin(w, r)
				return
			}

			if strings.HasPrefix(r.URL.Path, "/dashboard/api/kms") {
				d.HandleKMSAdmin(w, r)
				return
			}

			if strings.HasPrefix(r.URL.Path, "/dashboard/api/sqs") {
				d.HandleSQSAdmin(w, r)
				return
			}

			if strings.HasPrefix(r.URL.Path, "/dashboard/api/s3") {
				d.HandleS3Admin(w, r)
				return
			}

			if strings.HasPrefix(r.URL.Path, "/dashboard/api/logs") {
				d.HandleCloudWatchAdmin(w, r)
				return
			}

			return
		}

		// Handle Static Files
		d.HandleDashboard(w, r)
		return
	}

	amzTarget := r.Header.Get("X-Amz-Target")
	contentType := r.Header.Get("Content-Type")
	bodyValues := utils.ParseBody(r)
	action := bodyValues.Get("Action")

	if d.KmsSvc != nil &&
		strings.HasPrefix(amzTarget, "TrentService") {

		d.KmsSvc.Handle(w, r, amzTarget)
		return
	}

	if d.SmSvc != nil &&
		strings.HasPrefix(amzTarget, "secretsmanager") {

		d.SmSvc.Handle(w, r, amzTarget)
		return
	}

	if d.StsSvc != nil && strings.Contains(action, "GetCallerIdentity") {
		d.StsSvc.Handle(w, bodyValues)
		return
	}

	if contentType == "application/x-www-form-urlencoded" {
		if d.SqsSvc != nil && strings.HasPrefix(amzTarget, "AmazonSQS") {

			d.SqsSvc.Handle(w, r, amzTarget)
			return
		}
		if d.SnsSvc != nil && isSnsAction(action) {

			d.SnsSvc.Handle(w, bodyValues)
			return
		}
	}

	if d.S3Svc != nil && isS3Request(r) {

		d.S3Svc.Handle(w, r, amzTarget)
		return
	}

	d.Proxy.ServeHTTP(w, r)
}

func (d *Dispatcher) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	subFS, err := fs.Sub(d.UI, "ui")
	if err != nil {
		http.Error(w, "UI directory not found", 500)
		return
	}

	// 2. Wrap it in a FileServer
	server := http.FileServer(http.FS(subFS))

	// 3. Strip the "/dashboard" prefix
	// Important: Use "/dashboard/" with a trailing slash to handle sub-assets correctly
	handler := http.StripPrefix("/dashboard/", server)

	handler.ServeHTTP(w, r)
}

func (d *Dispatcher) HandleDashboardAPI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/dashboard/api/status" {
		resp := CombinedStatus{
			Services: health.ProbeInternalServices(d.CwSvc, utils.EnabledServices),
			Storage:  health.GetStorageStats(utils.VolumeDir),
			Volume:   utils.VolumeDir,
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			d.CwSvc.Error(SERVICE, "HandleDashboardAPI", fmt.Sprintf("Error encoding JSON: %v", err.Error()))
			return
		}

		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/dashboard/api/action" {
		var req struct {
			Action  string `json:"action"`
			Service string `json:"service"`
			Target  string `json:"target"` // e.g., bucket name or queue name
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad Request", 400)
			return
		}

		switch req.Action {
		case "flush_sqs":
			err := d.SqsSvc.FlushQueue(req.Target)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				d.CwSvc.Error(SERVICE, "HandleDashboardAPI:flush_sqs", fmt.Sprintf("Error clearing sqs queue: %s", err.Error()))
				return
			}
		case "clear_s3":
			err := d.S3Svc.ClearBucket(req.Target)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				d.CwSvc.Error(SERVICE, "HandleDashboardAPI:clear_s3", fmt.Sprintf("Error clearing s3 bucket: %s", err.Error()))
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		return
	}
}

func (d *Dispatcher) HandleLogStream(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	service := r.URL.Query().Get("service")

	logPath := utils.LogDir + "/edge.log"
	if service == "dynamodb" {
		logPath = utils.LogDir + "/dynamodb.log"
	}

	logChan := make(chan string)
	stopChan := make(chan struct{})
	defer close(stopChan)

	go utils.StreamLogFile(logPath, logChan, stopChan)

	// Flush the response to the client immediately
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case line := <-logChan:
			// SSE format requires "data: " prefix and double newline
			_, err := fmt.Fprintf(w, "data: %s\n\n", line)
			if err != nil {
				d.CwSvc.Error(SERVICE, "HandleLogStream", fmt.Sprintf("Error writing data: %s", err.Error()))
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			// Browser closed the connection
			return
		}
	}
}

func (d *Dispatcher) HandleLogDownload(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	logPath := utils.LogDir + "/edge.log"
	if service == "dynamodb" {
		logPath = utils.LogDir + "/dynamodb.log"
	}

	// Check if file exists before trying to serve it
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		http.Error(w, "Log file not found", http.StatusNotFound)
		return
	}

	// Force the browser to download instead of displaying
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.log", service))
	w.Header().Set("Content-Type", "text/plain")

	http.ServeFile(w, r, logPath)
}

func isS3Request(r *http.Request) bool {
	// S3 signatures usually contain "AWS4-HMAC-SHA256" and don't use X-Amz-Target
	return strings.Contains(r.Header.Get("Authorization"), "s3") ||
		r.Header.Get("x-amz-content-sha256") != ""
}

func isSnsAction(action string) bool {
	return utils.SnsActions[action]
}
