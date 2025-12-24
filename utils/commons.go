package utils

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

var DefaultDir = "/opt/cloudlocal"

func GetEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

var EnabledServices = strings.ToLower(GetEnv("ENABLED_SERVICES", ""))
var VolumeDir = strings.ToLower(GetEnv("CLOUDLOCAL_VOLUME_DIR", DefaultDir))
var AWS_REGION = strings.ToLower(GetEnv("AWS_REGION", "ap-southeast-2"))

func IsServiceEnabled(service string) bool {
	return strings.Contains(EnabledServices, strings.ToLower(service))
}

func UnmarshalJsonErrors(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func UnmarshalJson(data []byte, v any) {
	err := UnmarshalJsonErrors(data, v)
	if err != nil {
		return
	}
}

func MarshalJsonErrors(data any, indent bool) ([]byte, error) {
	if indent {
		return json.MarshalIndent(data, "", "  ")
	}
	return json.Marshal(data)
}

func MarshalIjson(data any) []byte {
	jsonData, err := MarshalJsonErrors(data, true)
	if err != nil {
		log.Fatalf("Error marshalling to JSON: %s", err)
	}

	return jsonData
}

type RespInput struct {
	Writer      http.ResponseWriter
	ContentType string
	Data        interface{}
}

func RespondByInput(input RespInput) {
	input.Writer.Header().Set("Content-Type", input.ContentType)
	err := json.NewEncoder(input.Writer).Encode(input.Data)
	if err != nil {
		return
	}
}

func RespondJSON(w http.ResponseWriter, data interface{}) {
	RespondByInput(RespInput{Writer: w, Data: data, ContentType: "application/x-amz-json-1.1"})
}

func ExtractFieldValues[T any, R any](objs []T, fieldMapper func(T) R) []R {
	// Pre-allocate the result slice for better performance
	result := make([]R, len(objs))

	for i, v := range objs {
		result[i] = fieldMapper(v)
	}

	return result
}
