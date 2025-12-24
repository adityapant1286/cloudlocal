package main

import (
	"cloudlocal/kms"
	"cloudlocal/servicediscovery"
	"cloudlocal/utils"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func main() {
	//env := os.Environ()
	//for k, v := range env {
	//	log.Println(k, "=", v)
	//}
	// Setup Proxy for DynamoDB (running internally on 10051)
	dynamoURL, _ := url.Parse("http://localhost:10051")
	proxy := httputil.NewSingleHostReverseProxy(dynamoURL)

	var kmsEnabled = utils.IsServiceEnabled("kms")
	var kmsSvc = kms.NewKmsService()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		if r.Method == "GET" && (r.URL.Path == "/health" || r.URL.Path == "/") {
			servicediscovery.Handle(w)
			return
		}

		target := r.Header.Get("X-Amz-Target")

		if strings.HasPrefix(target, "TrentService") && kmsEnabled {
			kmsSvc.Handle(w, r, target)
		} else {
			proxy.ServeHTTP(w, r)
		}
	})

	log.Println("CloudLocal Edge listening on :10050...")
	log.Fatal(http.ListenAndServe(":10050", nil))
}
