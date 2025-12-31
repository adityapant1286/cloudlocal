package utils

import (
	"encoding/json"
	"encoding/xml"
	"io"
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

// http://localhost:10050
var Port = "10050"
var CloudLocalUrl = "http://localhost:" + Port
var AccountId = "123456789012"
var EnabledServices = strings.ToLower(GetEnv("ENABLED_SERVICES", ""))
var VolumeDir = strings.ToLower(GetEnv("CLOUDLOCAL_VOLUME_DIR", DefaultDir))
var AwsRegion = strings.ToLower(GetEnv("AWS_REGION", "ap-southeast-2"))
var S3OwnerId = strings.ToLower(GetEnv("S3_OWNER_ID", "cloudlocal-s3-owner-id"))

type ServiceHandler interface {
	Handle(w http.ResponseWriter, r *http.Request, target string)
}

func IsServiceEnabled(service string) bool {
	return strings.Contains(EnabledServices, strings.ToLower(service))
}

func IsFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

func ToInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

/*
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
*/
type RespInput struct {
	Writer      http.ResponseWriter
	Code        int
	ContentType string
	Data        any
	ErrorStr    string
}

func RespondByInput(input RespInput) {
	//log.Printf("%s", MarshalIjson(input))
	input.Writer.Header().Set("Content-Type", input.ContentType)
	err := json.NewEncoder(input.Writer).Encode(input.Data)
	if err != nil {
		log.Fatalf("(X) Error: %s", err.Error())
		return
	}
}

func RespondJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		log.Fatalf("(X) Error: %s", err.Error())
		return
	}
}

func RespondXML(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/xml")
	_, err := w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	if err != nil {
		log.Fatalf("(X) Error: %s", err.Error())
		return
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ") // Makes it readable for debugging

	eerr := enc.Encode(data)
	if eerr != nil {
		log.Fatalf("(X) Error: %s", eerr.Error())
		return
	}
}

func DecodeXml(source io.Reader, target any) error {
	return xml.NewDecoder(source).Decode(target)
}

func RespondError(input RespInput) {
	data := map[string]any{
		"__type":  input.ErrorStr,
		"Code":    input.Code,
		"Message": input.Data,
	}
	log.Printf("%d|%s\n", input.Code, MarshalIjson(data))
	input.Writer.WriteHeader(input.Code)
	RespondJSON(input.Writer, data)
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
