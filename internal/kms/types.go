package kms

import "sync"

type kmsServiceImplementation struct {
	kms internalKms
}

// KmsKey represents our key metadata
type KmsKey struct {
	KeyId        string `json:"KeyId"`
	Arn          string `json:"Arn"`
	Description  string `json:"Description"`
	Enabled      bool   `json:"Enabled"`
	CreationDate int64  `json:"CreationDate"` // Unix timestamp
	KeyUsage     string `json:"KeyUsage"`
	KeyState     string `json:"KeyState"`
}

type Alias struct {
	AliasName   string `json:"AliasName"`
	TargetKeyId string `json:"TargetKeyId"`
}

// kmsImplementation (The "Class" with state)
type kmsImplementation struct {
	mu           sync.RWMutex
	keys         map[string]*KmsKey
	aliases      map[string]string // AliasName -> KeyID
	masterSecret []byte            // Used for the AES-GCM encryption
	storagePath  string            // Path to kms_state.json
}

type persistentState struct {
	Keys    map[string]*KmsKey `json:"keys"`
	Aliases map[string]string  `json:"aliases"`
}
