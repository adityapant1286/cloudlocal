package dispatcher

import (
	"cloudlocal/internal/health"
	"cloudlocal/internal/s3"
	"cloudlocal/internal/sqs"
	"cloudlocal/internal/utils"
	"embed"
	"net/http"
)

type Dispatcher struct {
	KmsSvc utils.ServiceHandler
	SmSvc  utils.ServiceHandler
	SqsSvc sqs.ServiceHandler
	S3Svc  s3.ServiceHandler
	UI     embed.FS
	Proxy  http.Handler // DynamoDB Proxy
}

type CombinedStatus struct {
	Services []health.ServiceStatus `json:"services"`
	Storage  map[string]string      `json:"storage"`
	Volume   string                 `json:"volume_path"`
}
