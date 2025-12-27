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
	Proxy  http.Handler // DynamoDB Proxy
}

func (d *Dispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" && (r.URL.Path == "/health" || r.URL.Path == "/") {
		servicediscovery.Handle(w)
		return
	}

	amzTarget := r.Header.Get("X-Amz-Target")

	if strings.HasPrefix(amzTarget, "TrentService") && d.KmsSvc != nil {
		d.KmsSvc.Handle(w, r, amzTarget)
		return
	}

	if strings.HasPrefix(amzTarget, "secretsmanager") && d.SmSvc != nil {
		d.SmSvc.Handle(w, r, amzTarget)
		return
	}

	d.Proxy.ServeHTTP(w, r)
}
