package secretsmanager

import (
	"cloudlocal/internal/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func NewSecretManagerService() utils.ServiceHandler {
	var smEnabled = utils.IsServiceEnabled("secretsmanager")

	if smEnabled {
		return &smImplementation{
			sm: newSecretsService(),
		}
	}
	return nil
}

func (svc *smImplementation) Handle(w http.ResponseWriter, r *http.Request, target string) {
	body, _ := io.ReadAll(r.Body)

	if strings.HasSuffix(target, "CreateSecret") {
		var req struct {
			Name         string `json:"Name"`
			SecretString string `json:"SecretString"`
			Description  string `json:"Description"`
		}
		utils.UnmarshalJson(body, &req)
		secret, err := svc.sm.createSecret(req.Name, req.Description, req.SecretString)
		if err != nil {
			utils.RespondError(utils.RespInput{
				Writer:   w,
				Code:     http.StatusBadRequest,
				ErrorStr: "ResourceNotCreatedException",
				Data:     map[string]string{"message": err.Error()},
			})
			return
		}
		utils.RespondJSON(w, secret)

	} else if strings.HasSuffix(target, "GetSecretValue") {
		var req struct{ SecretId string }
		utils.UnmarshalJson(body, &req)
		secret, err := svc.sm.getSecretValue(req.SecretId)
		if err != nil {
			utils.RespondError(utils.RespInput{
				Writer:   w,
				Code:     http.StatusNotFound,
				ErrorStr: "ResourceNotFoundException",
				Data:     map[string]string{"message": err.Error()},
			})
			return
		}
		utils.RespondJSON(w, secret)
	} else if strings.HasSuffix(target, "UpdateSecret") {
		var req struct {
			SecretId     string
			SecretString string
		}
		utils.UnmarshalJson(body, &req)
		err := svc.sm.updateSecret(req.SecretId, req.SecretString)
		if err != nil {
			utils.RespondError(utils.RespInput{
				Writer:   w,
				Code:     http.StatusNotFound,
				ErrorStr: "ResourceNotFoundException",
				Data:     map[string]string{"message": err.Error()},
			})
			return
		}
		// AWS returns ARN and Name on update
		utils.RespondJSON(w, map[string]string{"Name": req.SecretId})

	} else if strings.HasSuffix(target, "DeleteSecret") {
		var req struct {
			SecretId string `json:"SecretId"`
		}
		utils.UnmarshalJson(body, &req)
		err := svc.sm.deleteSecret(req.SecretId)
		if err != nil {
			utils.RespondError(utils.RespInput{
				Writer:   w,
				Code:     http.StatusNotFound,
				ErrorStr: "ResourceNotFoundException",
				Data:     map[string]string{"message": err.Error()},
			})
			return
		}
		utils.RespondJSON(w, map[string]string{"Name": req.SecretId, "DeletionDate": fmt.Sprintf("%d", time.Now().Unix())})

	} else if strings.HasSuffix(target, "DescribeSecret") {
		var req struct {
			SecretId string `json:"SecretId"`
		}
		utils.UnmarshalJson(body, &req)
		secret, err := svc.sm.describeSecret(req.SecretId)
		if err != nil {
			utils.RespondError(utils.RespInput{
				Writer:   w,
				Code:     http.StatusNotFound,
				ErrorStr: "ResourceNotFoundException",
				Data:     map[string]string{"message": err.Error()},
			})
			return
		}
		utils.RespondJSON(w, secret)

	} else if strings.HasSuffix(target, "ListSecrets") {
		var req struct {
			Filters   []Filter `json:"Filters"`
			SortBy    string   `json:"SortBy"`
			SortOrder string   `json:"SortOrder"`
		}
		utils.UnmarshalJson(body, &req)
		secrets, err := svc.sm.listSecrets(req.Filters, req.SortBy, req.SortOrder)
		if err != nil {
			utils.RespondError(utils.RespInput{
				Writer:   w,
				Code:     http.StatusNotFound,
				ErrorStr: "ResourceNotFoundException",
				Data:     map[string]string{"message": err.Error()},
			})
			return
		}
		utils.RespondJSON(w, map[string]interface{}{
			"SecretList": secrets,
		})
	}
}

type internalSecretsService interface {
	createSecret(name, description, value string) (*Secret, error)
	getSecretValue(name string) (*Secret, error)
	updateSecret(name, value string) error
	deleteSecret(name string) error
	describeSecret(name string) (*Secret, error)
	listSecrets(filters []Filter, sortBy, sortOrder string) ([]Secret, error)
}

func newSecretsService() internalSecretsService {
	smDir := filepath.Join(utils.VolumeDir, "secrets")
	path := filepath.Join(smDir, "secrets_state.json")
	if err := os.MkdirAll(smDir, 0755); err != nil {
		log.Fatalf("Critical: Could not create secrets directory: %v", err)
	}

	svc := &secretsImplementation{
		store:       make(map[string]*Secret),
		storagePath: path,
	}
	svc.load()
	return svc
}

func (s *secretsImplementation) save() {
	data, _ := json.MarshalIndent(s.store, "", "  ")
	err := os.WriteFile(s.storagePath, data, 0644)
	if err != nil {
		log.Printf("Error saving KMS state: %s", err)
	}
}

func (s *secretsImplementation) load() {
	data, err := os.ReadFile(s.storagePath)
	if err != nil {
		return // File doesn't exist yet, which is fine
	}
	var state map[string]*Secret
	if err := utils.UnmarshalJsonErrors(data, &state); err == nil {
		s.store = state
	}
}

func (s *secretsImplementation) createSecret(name, desc, value string) (*Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	region := os.Getenv("AWS_REGION")
	arn := fmt.Sprintf("arn:aws:secretsmanager:%s:123456789012:secret:%s", region, name)

	now := time.Now().Unix()
	secret := &Secret{
		Name:         name,
		ARN:          arn,
		Description:  desc,
		SecretString: value,
		CreatedDate:  now,
		LastChanged:  now,
	}

	s.store[name] = secret
	s.save()

	sc := &Secret{
		Name:        secret.Name,
		ARN:         secret.ARN,
		Description: secret.Description,
		CreatedDate: secret.CreatedDate,
		LastChanged: secret.LastChanged,
		Tags:        secret.Tags,
	}
	log.Printf("Created secret:\n%s\n\n", utils.MarshalIjson(sc))

	return sc, nil
}

func (s *secretsImplementation) getSecretValue(name string) (*Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.store[name]
	if !ok {
		return nil, errors.New("ResourceNotFoundException")
	}
	log.Printf("Retrieved secret:\n%s\n\n", utils.MarshalIjson(val))
	return val, nil
}

func (s *secretsImplementation) updateSecret(name, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	secret, ok := s.store[name]
	if !ok {
		return errors.New("ResourceNotFoundException: Secret not found")
	}

	secret.SecretString = value
	secret.LastChanged = time.Now().Unix()

	s.save()
	log.Printf("Updated secret:\n%s\n\n", name)
	return nil
}

func (s *secretsImplementation) deleteSecret(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.store[name]; !ok {
		return errors.New("ResourceNotFoundException: Secret not found")
	}

	delete(s.store, name)
	s.save()
	log.Printf("Deleted secret:\n%s\n\n", name)
	return nil
}

func (s *secretsImplementation) describeSecret(name string) (*Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	secret, ok := s.store[name]
	if !ok {
		return nil, errors.New("ResourceNotFoundException: Secret not found")
	}

	sc := &Secret{
		Name:        secret.Name,
		ARN:         secret.ARN,
		Description: secret.Description,
		CreatedDate: secret.CreatedDate,
		LastChanged: secret.LastChanged,
		Tags:        secret.Tags,
	}
	log.Printf("Retrieved secret:\n%s\n\n", utils.MarshalIjson(sc))

	return sc, nil
}

func (s *secretsImplementation) listSecrets(filters []Filter, sortBy, sortOrder string) ([]Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	secrets := slices.Collect(maps.Values(s.store))
	filtered := filterSecrets(filters, secrets)
	filtered = utils.SortArr(filtered, secretSortFunc(sortBy, sortOrder))

	log.Printf("Retrieved secrets:\n%s\n\n", utils.MarshalIjson(filtered))

	return filtered, nil
}

func filterSecrets(filters []Filter, secrets []*Secret) []Secret {

	filtered := make([]*Secret, len(secrets))
	copy(filtered, secrets)

	for _, f := range filters {
		filtered = utils.FilterArr(filtered, secretPredicate(f))
	}
	result := make([]Secret, len(filtered))

	for i, v := range filtered {
		result[i] = *v
	}

	return result
}

func secretPredicate(f Filter) func(*Secret) bool {
	if len(f.Values) == 0 {
		return func(secret *Secret) bool { return true }
	}
	if f.Key == "Name" {
		return func(secret *Secret) bool {
			for _, val := range f.Values {
				if strings.Contains(strings.ToLower(secret.Name), strings.ToLower(val.(string))) {
					return true
				}
			}
			return false
		}
	} else if f.Key == "Description" {
		return func(secret *Secret) bool {
			if len(secret.Description) > 0 {
				for _, val := range f.Values {
					if strings.Contains(strings.ToLower(secret.Description), strings.ToLower(val.(string))) {
						return true
					}
				}
			}
			return false
		}
	} else if f.Key == "CreatedDate" {
		return func(secret *Secret) bool {
			for _, val := range f.Values {
				if ts, ok := val.(float64); ok {
					if secret.CreatedDate == int64(ts) {
						return true
					}
				}
			}
			return false
		}
	} else if f.Key == "LastChanged" {
		return func(secret *Secret) bool {
			for _, val := range f.Values {
				if ts, ok := val.(float64); ok {
					if secret.LastChanged == int64(ts) {
						return true
					}
				}
			}
			return false
		}
	}

	return func(secret *Secret) bool { return true }
}

func secretSortFunc(sortBy, sortOrder string) func(Secret, Secret) bool {
	if sortBy == "Arn" {
		return func(s1, s2 Secret) bool {
			if sortOrder == "desc" {
				return strings.Compare(s1.ARN, s2.ARN) > 0
			}
			return strings.Compare(s1.ARN, s2.ARN) < 0
		}
	} else if sortBy == "CreatedDate" {
		return func(s1, s2 Secret) bool {
			if sortOrder == "desc" {
				return s1.CreatedDate > s2.CreatedDate
			}
			return s1.CreatedDate < s2.CreatedDate
		}
	} else if sortBy == "LastChangedDate" {
		return func(s1, s2 Secret) bool {
			if sortOrder == "desc" {
				return s1.LastChanged > s2.LastChanged
			}
			return s1.LastChanged < s2.LastChanged
		}
	}

	return func(s1, s2 Secret) bool {
		if sortOrder == "desc" {
			return strings.Compare(s1.Name, s2.Name) > 0
		}
		return strings.Compare(s1.Name, s2.Name) < 0
	}
}
