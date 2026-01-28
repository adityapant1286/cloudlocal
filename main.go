package main

import (
	"cloudlocal/internal/cloudwatch"
	"cloudlocal/internal/dispatcher"
	"cloudlocal/internal/kms"
	"cloudlocal/internal/lambda"
	"cloudlocal/internal/s3"
	"cloudlocal/internal/secretsmanager"
	"cloudlocal/internal/sns"
	"cloudlocal/internal/sqs"
	"cloudlocal/internal/sts"
	"cloudlocal/internal/utils"
	"embed"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

//go:embed ui/*
var uiFiles embed.FS

func main() {

	cw := cloudwatch.GetServiceInstance()
	sqsService := sqs.NewSQSService()

	appDispatcher := &dispatcher.Dispatcher{
		KmsSvc:    kms.NewKmsService(),
		SmSvc:     secretsmanager.NewSecretManagerService(),
		SqsSvc:    sqsService,
		S3Svc:     s3.NewS3Service(),
		SnsSvc:    sns.NewSnsService(sqsService),
		StsSvc:    sts.NewStsService(),
		LambdaSvc: lambda.NewLambdaService(),
		CwSvc:     cw,
		UI:        uiFiles,
		Proxy:     createDynamoProxy(),
	}

	cw.Info("CloudLocal", "Startup", fmt.Sprintf("CloudLocal Edge listening on :%s...", utils.Port))
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
