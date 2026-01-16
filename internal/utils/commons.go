package utils

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
)

var DefaultDir = "/opt/cloudlocal-data"

const (
	ResetCc     = "\033[0m"
	RedCc       = "\033[31m"
	YellowCc    = "\033[33m"
	BlueCc      = "\033[94m"
	CyanCc      = "\033[36m"
	LightGrayCc = "\033[37m"
)

func GetEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

// http://localhost:10050
const Port = "10050"
const CloudLocalUrl = "http://localhost:" + Port
const AccountId = "123456789012"
const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var EnabledServices = strings.ToLower(GetEnv("ENABLED_SERVICES", ""))
var VolumeDir = DefaultDir + "/cloudlocal"
var CloudWatchConsoleLogEnabled = GetEnv("CLOUDWATCH_CONSOLE_LOG", "false") == "true"
var DebugLogEnabled = GetEnv("DEBUG_LOG", "false") == "true"
var AwsRegion = strings.ToLower(GetEnv("AWS_REGION", "ap-southeast-2"))
var S3OwnerId = strings.ToLower(GetEnv("S3_OWNER_ID", "cloudlocal-s3-owner-id"))
var DashboardEnabled = GetEnv("DISABLE_DASHBOARD", "false") != "true"
var LogDir = VolumeDir + "/logs"
var SnsActions = map[string]bool{"Publish": true, "CreateTopic": true, "Subscribe": true, "ListTopics": true}

type ServiceHandler interface {
	Handle(w http.ResponseWriter, r *http.Request, target string)
}

func ApiAuthHeader(service string) string {
	return fmt.Sprintf("AWS4-HMAC-SHA256 Credential=cloudlocal/20260101/%s/%s/aws4_request, SignedHeaders=host;x-amz-date;x-amz-target, Signature=dummy", AwsRegion, service)
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

func GenAlphanumeric(length int) string {
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

func RandomUuid() string {
	s := "1234abcd-1ABC-1234-abcd-%s"
	return fmt.Sprintf(s, GenAlphanumeric(12))
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

func ParseBody(r *http.Request) url.Values {
	// 1. Read the entire body into memory
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return url.Values{}
	}

	// 2. IMPORTANT: Put the body back!
	// Because we read it, r.Body is now empty. We must refill it
	// so the next handler (like the DynamoDB Proxy) isn't confused.
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// 3. Parse the bytes as a URL-encoded form
	values, err := url.ParseQuery(string(bodyBytes))
	if err != nil {
		return url.Values{}
	}

	return values
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
