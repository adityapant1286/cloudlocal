package servicediscovery

import (
	"cloudlocal/internal/cloudwatch"
	"cloudlocal/internal/utils"
	"fmt"
	"net/http"
)

func Handle(w http.ResponseWriter) {
	cw := cloudwatch.GetServiceInstance()
	var activeServices []ServiceStatus
	if utils.IsServiceEnabled("kms") {
		activeServices = append(activeServices, ServiceStatus{
			Name:   "KMS",
			Status: "running",
		})
	}

	// Check if DynamoDB is enabled via Env Var
	if utils.IsServiceEnabled("dynamodb") {
		activeServices = append(activeServices, ServiceStatus{
			Name:   "DynamoDB",
			Status: "running",
			Port:   10051,
		})
	}

	serviceNames := utils.ExtractFieldValues(
		activeServices,
		func(s ServiceStatus) string { return s.Name },
	)

	data := map[string]interface{}{
		"ok":       true,
		"services": serviceNames,
		"region":   utils.AwsRegion,
	}
	cw.Error(SERVICE, "ServiceDiscoveryHandler", fmt.Sprintf("Service Discovery resp: %s", utils.MarshalIjson(data)))

	utils.RespondByInput(utils.RespInput{
		Writer:      w,
		ContentType: "application/json",
		Data:        data,
	})

}
