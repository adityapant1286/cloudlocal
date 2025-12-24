package servicediscovery

import (
	"cloudlocal/utils"
	"log"
	"net/http"
)

type ServiceStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Port    int    `json:"port,omitempty"`
	Version string `json:"version,omitempty"`
}

func Handle(w http.ResponseWriter) {
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
		"region":   utils.AWS_REGION,
	}
	log.Printf("Service Discovery resp:\n%s\n\n", utils.MarshalIjson(data))

	utils.RespondByInput(utils.RespInput{
		Writer:      w,
		ContentType: "application/json",
		Data:        data,
	})

}
