package lambda

import (
	"cloudlocal/internal/cloudwatch"
	"sync"
)

const LAMBDA = "lambda"
const SERVICE = LAMBDA + "-service"

type InvokeRequest struct {
	FunctionName string `json:"FunctionName"`
	Payload      string `json:"payload"` // The JSON event
}

type Environment struct {
	Variables map[string]string `json:"Variables"`
}

type FunctionConfig struct {
	FunctionName string      `json:"FunctionName"`
	Runtime      string      `json:"Runtime"`
	Role         string      `json:"Role"`
	Handler      string      `json:"Handler"`
	RevisionId   string      `json:"RevisionId"`
	FunctionArn  string      `json:"FunctionArn"`
	Environment  Environment `json:"Environment"`
	LastModified int64       `json:"LastModified"`
}

type persistentState struct {
	Functions map[string]*FunctionConfig `json:"functions"`
}

type lambdaServiceImplementation struct {
	lambda internalLambda
	cw     cloudwatch.CwService
}

type lambdaSvcImplementation struct {
	mu          sync.RWMutex
	cw          cloudwatch.CwService
	functions   map[string]*FunctionConfig
	storagePath string
}
