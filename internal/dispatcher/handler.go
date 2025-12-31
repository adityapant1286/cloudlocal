package dispatcher

import (
	"cloudlocal/internal/servicediscovery"
	"cloudlocal/internal/utils"
	"net/http"
	"strings"
)

type Dispatcher struct {
	KmsSvc utils.ServiceHandler
	SmSvc  utils.ServiceHandler
	SqsSvc utils.ServiceHandler
	S3Svc  utils.ServiceHandler
	Proxy  http.Handler // DynamoDB Proxy
}

func (d *Dispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" && (r.URL.Path == "/health" || r.URL.Path == "/") {
		servicediscovery.Handle(w)
		return
	}

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

func isS3Request(r *http.Request) bool {
	// S3 signatures usually contain "AWS4-HMAC-SHA256" and don't use X-Amz-Target
	return strings.Contains(r.Header.Get("Authorization"), "s3") ||
		r.Header.Get("x-amz-content-sha256") != ""
}
