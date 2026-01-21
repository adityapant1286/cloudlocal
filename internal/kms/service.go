package kms

import (
	"cloudlocal/internal/cloudwatch"
	"cloudlocal/internal/utils"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func NewKmsService() utils.ServiceHandler {
	cw := cloudwatch.GetServiceInstance()
	var kmsEnabled = utils.IsServiceEnabled(KMS)

	if kmsEnabled {
		return &kmsServiceImplementation{
			kms: newKms(cw),
		}
	}
	return nil
}

func (svc *kmsServiceImplementation) Handle(w http.ResponseWriter, r *http.Request, target string) {
	body, _ := io.ReadAll(r.Body)

	var action = r.FormValue("Action")
	if action == "" {
		action = strings.ReplaceAll(target, "TrentService.", "")
	}

	switch action {
	case "DescribeKey":
		var req struct {
			KeyId string `json:"KeyId"`
		}
		utils.UnmarshalJson(body, &req)

		key, err := svc.kms.describeKey(req.KeyId)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			utils.RespondJSON(w, map[string]string{"message": err.Error()})
			return
		}

		utils.RespondJSON(w, map[string]interface{}{"KeyMetadata": key})
	case "CreateAlias":
		var req struct {
			AliasName   string `json:"AliasName"`
			TargetKeyId string `json:"TargetKeyId"`
		}
		utils.UnmarshalJson(body, &req)

		err := svc.kms.createAlias(req.AliasName, req.TargetKeyId)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			utils.RespondJSON(w, map[string]interface{}{"message": err.Error()})
			return
		}
		// AWS returns 200 OK with no body for CreateAlias
		w.WriteHeader(http.StatusOK)
	case "CreateKey":
		var req struct {
			Description string
		}
		utils.UnmarshalJson(body, &req)

		key, _ := svc.kms.createKey(req.Description)
		utils.RespondJSON(w, map[string]interface{}{"KeyMetadata": key})
	case "Encrypt":
		var req struct {
			KeyId     string
			Plaintext []byte
		}
		utils.UnmarshalJson(body, &req)
		blob, _ := svc.kms.encrypt(svc.kms.resolveKeyId(req.KeyId), req.Plaintext)
		utils.RespondJSON(w, map[string]string{"CiphertextBlob": blob})
	case "Decrypt":
		var req struct{ CiphertextBlob string }
		utils.UnmarshalJson(body, &req)
		plain, _ := svc.kms.decrypt(req.CiphertextBlob)
		utils.RespondJSON(w, map[string]interface{}{"Plaintext": plain})
	case "ListAliases":
		var req struct {
			KeyId string `json:"KeyId"`
		}
		utils.UnmarshalJson(body, &req)

		aliases, _ := svc.kms.listAliases(req.KeyId)
		utils.RespondJSON(w, map[string]interface{}{"Aliases": aliases})
	case "ListKeys":
		keys, _ := svc.kms.listKeys()
		utils.RespondJSON(w, map[string]interface{}{"Keys": keys})
	}
}

type internalKms interface {
	createAlias(aliasName string, targetKeyID string) error
	createKey(description string) (*KmsKey, error)
	decrypt(ciphertextBlob string) ([]byte, error)
	describeKey(keyIdOrAlias string) (*KmsKey, error)
	encrypt(keyId string, plaintext []byte) (string, error)
	listAliases(keyId string) ([]Alias, error)
	listKeys() ([]KmsKey, error)
	resolveKeyId(idOrAlias string) string
}

func newKms(cw cloudwatch.CwService) internalKms {

	secret := utils.GetEnv("MASTER_SECRET", "cloudlocal-secret-32-chars-long!")

	kmsDir := filepath.Join(utils.VolumeDir, KMS)
	path := filepath.Join(kmsDir, "kms_state.json")

	if err := os.MkdirAll(kmsDir, 0755); err != nil {
		cw.Error(SERVICE, "Config", fmt.Sprintf("Could not create KMS directory: %v", err))
	}

	svc := &kmsImplementation{
		cloudwatch:   cw,
		keys:         make(map[string]*KmsKey),
		aliases:      make(map[string]string),
		masterSecret: []byte(secret[:32]), // 32 bytes for AES-256
		storagePath:  path,
	}
	// Load existing state if available
	svc.load()
	return svc
}

func (s *kmsImplementation) save() {
	state := persistentState{
		Keys:    s.keys,
		Aliases: s.aliases,
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	err := os.WriteFile(s.storagePath, data, 0644)
	if err != nil {
		s.cloudwatch.Error(SERVICE, "KmsConfig", fmt.Sprintf("Error saving KMS state: %s", err))
	}
}

func (s *kmsImplementation) load() {
	data, err := os.ReadFile(s.storagePath)
	if err != nil {
		return // File doesn't exist yet, which is fine
	}
	var state persistentState
	if err := utils.UnmarshalJsonErrors(data, &state); err == nil {
		s.keys = state.Keys
		s.aliases = state.Aliases
	}
}

func (s *kmsImplementation) createAlias(aliasName string, targetKeyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// In AWS, aliases must start with "alias/"
	if !strings.HasPrefix(aliasName, "alias/") {
		return errors.New("alias must start with 'alias/'")
	}

	// Basic check: does the key exist? (optional for mock but helpful)
	// targetKeyID could be an ARN or a UUID, so we'd normally resolve it.
	s.aliases[aliasName] = targetKeyID
	s.save() // PERSIST
	s.cloudwatch.Info(SERVICE, "CreateAlias", fmt.Sprintf("Created KMS alias: %s", utils.MarshalIjson(Alias{AliasName: aliasName, TargetKeyId: targetKeyID})))
	return nil
}

func (s *kmsImplementation) createKey(description string) (*KmsKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("%d", len(s.keys)+1) // Simple ID generation
	keyId := fmt.Sprintf("1234abcd-12ab-34cd-56ef-12345678%04s", id)
	arn := fmt.Sprintf("arn:aws:kms:%s:%s:key/%s", utils.AwsRegion, utils.AccountId, keyId)

	newKey := &KmsKey{
		KeyId:        keyId,
		Arn:          arn,
		Description:  description,
		Enabled:      true,
		CreationDate: time.Now().Unix(),
		KeyUsage:     "ENCRYPT_DECRYPT",
		KeyState:     "Enabled",
	}
	s.keys[keyId] = newKey
	s.save() // PERSIST
	s.cloudwatch.Info(SERVICE, "CreateKey", fmt.Sprintf("Created KMS key: %s", utils.MarshalIjson(newKey)))
	return newKey, nil
}

func (s *kmsImplementation) decrypt(blob string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(blob)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(s.masterSecret)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	result, err := gcm.Open(nil, nonce, ciphertext, nil)
	s.cloudwatch.Info(SERVICE, "Decryption", fmt.Sprintf("Decrypted: %s", string(result)))
	return result, err
}

func (s *kmsImplementation) describeKey(keyIdOrAlias string) (*KmsKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. Resolve alias if necessary
	keyId := s.resolveKeyId(keyIdOrAlias)

	// 2. Look up the key
	key, exists := s.keys[keyId]
	if !exists {
		return nil, errors.New("NotFoundException: Key not found")
	}

	s.cloudwatch.Info(SERVICE, "DescribeKey", fmt.Sprintf("Found KMS key: %s", utils.MarshalIjson(key)))

	return key, nil
}

func (s *kmsImplementation) encrypt(keyId string, plaintext []byte) (string, error) {
	if s.keys[keyId] == nil {
		return "", errors.New("KeyNotFoundException: The specified key does not exist")
	}

	s.cloudwatch.Info(SERVICE, "Encryption", fmt.Sprintf("Encrypting with KeyID: %s", keyId))

	block, err := aes.NewCipher(s.masterSecret)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Sealed data: nonce + ciphertext
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	result := base64.StdEncoding.EncodeToString(ciphertext)

	s.cloudwatch.Info(SERVICE, "Encryption", fmt.Sprintf("Encrypted: %s", result))
	return result, nil
}

func (s *kmsImplementation) listAliases(keyId string) ([]Alias, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Alias
	if keyId != "" {
		for name, kId := range s.aliases {
			if keyId == kId {
				result = append(result, Alias{AliasName: name, TargetKeyId: kId})
			}
		}
	} else {
		for name, kId := range s.aliases {
			result = append(result, Alias{AliasName: name, TargetKeyId: kId})
		}
	}
	s.cloudwatch.Info(SERVICE, "ListAliases", fmt.Sprintf("KMS aliases: %s", utils.MarshalIjson(result)))
	return result, nil
}

func (s *kmsImplementation) listKeys() ([]KmsKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []KmsKey
	for _, key := range s.keys {
		result = append(result, *key)
	}
	s.cloudwatch.Info(SERVICE, "ListKeys", fmt.Sprintf("KMS keys: %s", utils.MarshalIjson(result)))
	return result, nil
}

func (s *kmsImplementation) resolveKeyId(idOrAlias string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if strings.HasPrefix(idOrAlias, "alias/") {
		if keyId, ok := s.aliases[idOrAlias]; ok {
			return keyId
		}
	}
	return idOrAlias
}
