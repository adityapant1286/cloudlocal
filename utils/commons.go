package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
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

func AnyToString(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v) // Use strconv for efficient numeric conversion
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprint(v) // Fallback for other types
	}
}

type RespInput struct {
	Writer      http.ResponseWriter
	Code        int
	ContentType string
	Data        any
	ErrorStr    string
}

func RespondByInput(input RespInput) {
	input.Writer.Header().Set("Content-Type", input.ContentType)
	err := json.NewEncoder(input.Writer).Encode(input.Data)
	if err != nil {
		return
	}
}

func RespondJSON(w http.ResponseWriter, data interface{}) {
	RespondByInput(RespInput{
		Writer:      w,
		Data:        data,
		ContentType: "application/x-amz-json-1.1",
	})
}

func RespondError(input RespInput) {
	input.Writer.WriteHeader(input.Code)
	RespondJSON(input.Writer, map[string]string{
		"__type":  input.ErrorStr,
		"Message": AnyToString(input.Data),
	})
}

func ExtractFieldValues[T any, R any](objs []T, fieldMapper func(T) R) []R {
	// Pre-allocate the result slice for better performance
	result := make([]R, len(objs))

	for i, v := range objs {
		result[i] = fieldMapper(v)
	}

	return result
}

func FilterArr[T any](arr []T, predicate func(T) bool) []T {
	var result []T
	for _, v := range arr {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

func SortArr[T any](arr []T, compare func(T, T) bool) []T {
	sort.Slice(arr, func(i, j int) bool {
		return compare(arr[i], arr[j])
	})
	return arr
}
