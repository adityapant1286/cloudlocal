package secretsmanager

import (
	"cloudlocal/internal/cloudwatch"
	"sync"
)

const SECRETS_MANAGER = "secretsmanager"
const SERVICE = SECRETS_MANAGER + "-service"

type smImplementation struct {
	sm internalSecretsService
}

type Secret struct {
	Name         string            `json:"Name"`
	ARN          string            `json:"ARN"`
	Description  string            `json:"Description"`
	SecretString string            `json:"SecretString,omitempty"`
	Tags         map[string]string `json:"Tags,omitempty"`
	CreatedDate  int64             `json:"CreatedDate"`
	LastChanged  int64             `json:"LastChangedDate"`
}

type secretsImplementation struct {
	mu          sync.RWMutex
	cloudwatch  cloudwatch.CwService
	store       map[string]*Secret
	storagePath string
}

type Filter struct {
	Key    string `json:"Key"`
	Values []any  `json:"Values"`
}
