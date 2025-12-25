package main

import (
	"cloudlocal/kms"
	"cloudlocal/secretsmanager"
	"cloudlocal/servicediscovery"
	"cloudlocal/utils"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func main() {
	dynamoURL, _ := url.Parse("http://localhost:10051")
	proxy := httputil.NewSingleHostReverseProxy(dynamoURL)

	var kmsEnabled = utils.IsServiceEnabled("kms")
	var kmsSvc kms.KmsService = nil
	if kmsEnabled {
		kmsSvc = kms.NewKmsService()
	}
	var smEnabled = utils.IsServiceEnabled("secretsmanager")
	var smSvc secretsmanager.SecretManagerService = nil
	if smEnabled {
		smSvc = secretsmanager.NewSecretManagerService()
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		if r.Method == "GET" && (r.URL.Path == "/health" || r.URL.Path == "/") {
			servicediscovery.Handle(w)
			return
		}

		target := r.Header.Get("X-Amz-Target")

		if strings.HasPrefix(target, "TrentService") && kmsEnabled {
			kmsSvc.Handle(w, r, target)
		} else if strings.HasPrefix(target, "secretsmanager") && smEnabled {
			smSvc.Handle(w, r, target)
		} else {
			proxy.ServeHTTP(w, r)
		}
	})

	log.Println("CloudLocal Edge listening on :10050...")
	log.Fatal(http.ListenAndServe(":10050", nil))
}
