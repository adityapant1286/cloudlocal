package main

import (
	"cloudlocal/internal/dispatcher"
	"cloudlocal/internal/kms"
	"cloudlocal/internal/s3"
	"cloudlocal/internal/secretsmanager"
	"cloudlocal/internal/sqs"
	"cloudlocal/internal/utils"
	"embed"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

//go:embed ui/*
var uiFiles embed.FS

func main() {

	appDispatcher := &dispatcher.Dispatcher{
		KmsSvc: kms.NewKmsService(),
		SmSvc:  secretsmanager.NewSecretManagerService(),
		SqsSvc: sqs.NewSQSService(),
		S3Svc:  s3.NewS3Service(),
		UI:     uiFiles,
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
