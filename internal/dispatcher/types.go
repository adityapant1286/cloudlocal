package dispatcher

import (
	"cloudlocal/internal/cloudwatch"
	"cloudlocal/internal/health"
	"cloudlocal/internal/s3"
	"cloudlocal/internal/sns"
	"cloudlocal/internal/sqs"
	"cloudlocal/internal/sts"
	"cloudlocal/internal/utils"
	"embed"
	"net/http"
)

const SERVICE = "dispatcher-service"

type Dispatcher struct {
	KmsSvc    utils.ServiceHandler
	SmSvc     utils.ServiceHandler
	SqsSvc    sqs.ServiceHandler
	S3Svc     s3.ServiceHandler
	SnsSvc    sns.ServiceHandler
	StsSvc    sts.ServiceHandler
	LambdaSvc utils.ServiceHandler
	CwSvc     cloudwatch.CwService
	UI        embed.FS
	Proxy     http.Handler // DynamoDB Proxy
}

type CombinedStatus struct {
	Services []health.ServiceStatus `json:"services"`
	Storage  map[string]string      `json:"storage"`
	Volume   string                 `json:"volume_path"`
}
