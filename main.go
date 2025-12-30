package main

import (
	"cloudlocal/internal/dispatcher"
	"cloudlocal/internal/kms"
	"cloudlocal/internal/secretsmanager"
	"cloudlocal/internal/sqs"
	"cloudlocal/internal/utils"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {

	var smEnabled = utils.IsServiceEnabled("secretsmanager")
	var smSvc utils.ServiceHandler = nil
	if smEnabled {
		smSvc = secretsmanager.NewSecretManagerService()
	}

	appDispatcher := &dispatcher.Dispatcher{
		KmsSvc: kms.NewKmsService(),
		SmSvc:  smSvc,
		SqsSvc: sqs.NewSQSService(),
		Proxy:  createDynamoProxy(),
	}

	log.Printf("CloudLocal Edge listening on :%s...", utils.Port)
	log.Fatal(http.ListenAndServe(":10050", appDispatcher))
}

func createDynamoProxy() http.Handler {
	proxyURL, _ := url.Parse("http://localhost:10051")
	proxy := httputil.NewSingleHostReverseProxy(proxyURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = proxyURL.Host
	}
	return proxy
}
