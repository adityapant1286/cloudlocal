package dispatcher

import (
	"cloudlocal/internal/health"
	"cloudlocal/internal/utils"
	"encoding/json"
	"io/fs"
	"net/http"
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
			d.HandleDashboardAPI(w, r)
			return
		}

		// Handle Static Files
		d.HandleDashboard(w, r)
		return
	}

	//if r.Method == "GET" && (r.URL.Path == "/health" || r.URL.Path == "/") {
	//	servicediscovery.Handle(w)
	//	return
	//}

	amzTarget := r.Header.Get("X-Amz-Target")
	contentType := r.Header.Get("Content-Type")

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

	if d.SqsSvc != nil &&
		(strings.HasPrefix(amzTarget, "AmazonSQS") ||
			contentType == "application/x-www-form-urlencoded") {

		d.SqsSvc.Handle(w, r, amzTarget)
		return
	}

	if d.S3Svc != nil && amzTarget == "" && isS3Request(r) {

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
			Services: health.ProbeInternalServices(utils.EnabledServices),
			Storage:  health.GetStorageStats(utils.VolumeDir),
			Volume:   utils.VolumeDir,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

		return
	}
}

func isS3Request(r *http.Request) bool {
	// S3 signatures usually contain "AWS4-HMAC-SHA256" and don't use X-Amz-Target
	return strings.Contains(r.Header.Get("Authorization"), "s3") ||
		r.Header.Get("x-amz-content-sha256") != ""
}
