package s3

import (
	"cloudlocal/internal/cloudwatch"
	"sync"
	"time"
)

const S3 = "s3"
const SERVICE = S3 + "-service"

type s3ServiceImplementation struct {
	cloudwatch cloudwatch.CwService
	s3         internalS3Service
}

type Bucket struct {
	Name         string    `json:"Name"`
	CreationDate time.Time `json:"CreationDate"`
	Path         string    `json:"Path"`
}

type ListBucketsResponse struct {
	//XMLName xml.Name `json:"ListAllMyBucketsResult"`
	//Xmlns   string   `json:"xmlns,attr"`
	Owner   Owner    `json:"Owner"`
	Buckets []Bucket `json:"Buckets"`
}

type Owner struct {
	ID          string `json:"ID"`
	DisplayName string `json:"DisplayName"`
}

type ListObjectsV2Response struct {
	//XMLName        xml.Name `json:"ListBucketResult"`
	//Xmlns          string   `json:"xmlns,attr"`
	Name           string   `json:"Name"` // Bucket name
	Prefix         string   `json:"Prefix"`
	KeyCount       int      `json:"KeyCount"`
	MaxKeys        int      `json:"MaxKeys"`
	IsTruncated    bool     `json:"IsTruncated"`
	Contents       []Object `json:"Contents"`
	CommonPrefixes []Prefix `json:"CommonPrefixes,omitempty"`
}

type Object struct {
	Key          string    `json:"Key"`
	LastModified time.Time `json:"LastModified"`
	ETag         string    `json:"ETag"`
	Size         int64     `json:"Size"`
	StorageClass string    `json:"StorageClass"`
}

type Prefix struct {
	Prefix string `json:"Prefix"`
}

type DeleteRequest struct {
	//XMLName xml.Name           `json:"Delete"`
	Objects []ObjectIdentifier `json:"Object"`
	Quiet   bool               `json:"Quiet"`
}

type ObjectIdentifier struct {
	Key string `json:"Key"`
}

type DeleteResult struct {
	//XMLName xml.Name        `json:"DeleteResult"`
	//Xmlns   string          `json:"xmlns,attr"`
	Deleted []DeletedObject `json:"Deleted"`
}

type DeletedObject struct {
	Key string `json:"Key"`
}

type MultipartUpload struct {
	UploadID string
	Bucket   string
	Key      string
	Parts    map[int]string // PartNumber -> ETag
}

type InitiateMultipartUploadResult struct {
	//XMLName  xml.Name `json:"InitiateMultipartUploadResult"`
	//Xmlns    string `json:"xmlns,attr"`
	Bucket   string `json:"Bucket"`
	Key      string `json:"Key"`
	UploadId string `json:"UploadId"`
}

type CompleteMultipartUploadRequest struct {
	//XMLName xml.Name        `json:"CompleteMultipartUpload"`
	Parts []CompletedPart `json:"Part"`
}

type CompletedPart struct {
	PartNumber int    `json:"PartNumber"`
	ETag       string `json:"ETag"`
}

type CompleteMultipartUploadResult struct {
	//XMLName  xml.Name `json:"CompleteMultipartUploadResult"`
	Location string `json:"Location"`
	Bucket   string `json:"Bucket"`
	Key      string `json:"Key"`
	ETag     string `json:"ETag"`
}

type s3Implementation struct {
	mu            sync.RWMutex
	storagePath   string
	region        string
	cloudwatch    cloudwatch.CwService
	activeUploads map[string]*MultipartUpload
	buckets       map[string]*Bucket
}
