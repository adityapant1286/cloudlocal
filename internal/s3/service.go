package s3

import (
	"cloudlocal/internal/utils"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type BucketClearer interface {
	ClearBucket(bucketName string) error
}

type ServiceHandler interface {
	utils.ServiceHandler
	BucketClearer
}

func NewS3Service() ServiceHandler {
	var s3Enabled = utils.IsServiceEnabled("s3")

	if s3Enabled {
		return &s3ServiceImplementation{
			s3: newS3Service(),
		}
	}
	return nil
}

func (svc *s3ServiceImplementation) Handle(w http.ResponseWriter, r *http.Request, target string) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	log.Printf("%s", target)
	queryParams := r.URL.Query()
	if len(parts) == 0 || parts[0] == "" {
		// List Buckets (GET /)
		resp := svc.s3.listBuckets()
		// xml
		w.WriteHeader(http.StatusOK)
		utils.RespondXML(w, resp)
		return
	}

	bucket := parts[0]
	key := strings.Join(parts[1:], "/")

	switch r.Method {
	case http.MethodPut:
		// 1. Upload Part: PUT /bucket/key?partNumber=X&uploadId=Y
		if queryParams.Has("uploadId") && queryParams.Has("partNumber") {
			uploadID := queryParams.Get("uploadId")
			partNum, _ := strconv.Atoi(queryParams.Get("partNumber"))

			etag, err := svc.s3.uploadPart(uploadID, partNum, r.Body)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				utils.RespondXML(w, map[string]any{
					"Error": err.Error(),
				})
				return
			}
			// S3 requires the ETag header for parts
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusOK)
			return
		}

		// 2. Create Bucket: PUT /bucket/ (Key is empty)
		if key == "" {
			if err := svc.s3.createBucket(bucket); err != nil {
				w.WriteHeader(http.StatusConflict)
				utils.RespondXML(w, map[string]any{
					"Error": err.Error(),
				})
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		// 3. Put Object: PUT /bucket/key
		if err := svc.s3.putObject(bucket, key, r.Body); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			utils.RespondXML(w, map[string]any{
				"Error": err.Error(),
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:

		if queryParams.Get("list-type") == "2" {
			prefix := queryParams.Get("prefix")
			resp, _ := svc.s3.listObjectsV2(bucket, prefix)
			w.WriteHeader(http.StatusOK)
			utils.RespondXML(w, resp)
			return
		}

		data, err := svc.s3.getObject(bucket, key)
		if err != nil {
			utils.RespondError(utils.RespInput{
				Writer:   w,
				Code:     http.StatusNotFound,
				ErrorStr: "NotFoundException",
				Data:     map[string]string{"message": err.Error()},
			})
			return
		}
		defer data.Close()
		io.Copy(w, data) // Stream file back to user
	case http.MethodDelete:
		// Single Delete: DELETE /bucket/key
		_ = svc.s3.deleteObject(bucket, key)
		w.WriteHeader(http.StatusNoContent) // No Content is standard for S3 Delete
	case http.MethodPost:
		// Initiate Multipart: POST /bucket/key?uploads
		if queryParams.Has("uploads") {
			uploadID := fmt.Sprintf("upload-%d", time.Now().UnixNano())
			err := svc.s3.initMultipartUpload(bucket, key, uploadID)
			if err != nil {
				w.WriteHeader(http.StatusUnprocessableEntity)
				utils.RespondXML(w, map[string]any{
					"Error": err.Error(),
				})
				return
			}

			w.WriteHeader(http.StatusOK)
			utils.RespondXML(w, InitiateMultipartUploadResult{
				Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
				Bucket:   bucket,
				Key:      key,
				UploadId: uploadID,
			})
			return
		}

		// Complete Multipart: POST /bucket/key?uploadId=X
		if queryParams.Has("uploadId") {
			res, err := svc.s3.completeMultipartUpload(
				bucket, key,
				queryParams.Get("uploadId"),
				r.Body,
			)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				utils.RespondXML(w, map[string]any{
					"Error": err.Error(),
				})
				return
			}
			w.WriteHeader(http.StatusOK)
			utils.RespondXML(w, res)
			return
		}

		// Bulk Delete: POST /bucket?delete
		if queryParams.Has("delete") {
			resp := DeleteResult{
				Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
			}

			var delReq DeleteRequest
			err := utils.DecodeXml(r.Body, &delReq)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				utils.RespondXML(w, map[string]any{
					"Error": err.Error(),
				})
				return
			}

			var keys []string
			for _, obj := range delReq.Objects {
				keys = append(keys, obj.Key)
			}

			deleted, err := svc.s3.deleteObjects(bucket, keys)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				utils.RespondXML(w, map[string]any{
					"Error": err.Error(),
				})
				return
			}

			for _, k := range deleted {
				resp.Deleted = append(resp.Deleted, DeletedObject{Key: k})
			}
			w.WriteHeader(http.StatusOK)
			utils.RespondXML(w, resp)
		}
	}
}

func (svc *s3ServiceImplementation) ClearBucket(bucketName string) error {
	return svc.s3.clearBucket(bucketName)
}

type internalS3Service interface {
	createBucket(name string) error
	deleteBucket(name string) error
	listBuckets() ListBucketsResponse
	putObject(bucket, key string, data io.Reader) error
	getObject(bucket, key string) (io.ReadCloser, error)
	listObjectsV2(bucket, prefix string) (ListObjectsV2Response, error)
	deleteObject(bucket, key string) error
	deleteObjects(bucket string, keys []string) ([]string, error)
	initMultipartUpload(bucket, key, uploadID string) error
	uploadPart(uploadID string, partNum int, data io.Reader) (string, error)
	completeMultipartUpload(bucket, key, uploadID string, body io.Reader) (CompleteMultipartUploadResult, error)
	clearBucket(bucketName string) error
}

func newS3Service() internalS3Service {
	s3Dir := filepath.Join(utils.VolumeDir, "s3")

	if err := os.MkdirAll(s3Dir, 0755); err != nil {
		log.Fatalf("Critical: Could not create S3 directory: %v", err)
	}

	svc := &s3Implementation{
		storagePath:   s3Dir,
		region:        utils.AwsRegion,
		activeUploads: make(map[string]*MultipartUpload),
		buckets:       make(map[string]*Bucket),
	}
	svc.load()

	return svc
}

func (s *s3Implementation) load() {
	entries, err := os.ReadDir(s.storagePath)
	if err != nil {
		return // File doesn't exist yet, which is fine
	}

	buckets := make(map[string]*Bucket)
	for _, entry := range entries {
		if entry.IsDir() {
			info, _ := entry.Info()
			buckets[entry.Name()] = &Bucket{
				Name:         entry.Name(),
				CreationDate: info.ModTime(),
				Path:         filepath.Join(s.storagePath, entry.Name()),
			}
		}
	}
}

func (s *s3Implementation) createBucket(name string) error {
	bucketPath := filepath.Join(s.storagePath, name)
	if !utils.IsFileExists(bucketPath) {
		log.Printf("Creating S3 bucket:\n%s\n\n", name)
		s.buckets[name] = &Bucket{
			Name:         name,
			CreationDate: time.Now(),
			Path:         bucketPath,
		}
	}
	return os.MkdirAll(bucketPath, 0755)
}

func (s *s3Implementation) deleteBucket(name string) error {
	bucketPath := filepath.Join(s.storagePath, name)
	log.Printf("Deleting S3 bucket:\n%s\n\n", name)
	delete(s.buckets, name)
	return os.RemoveAll(bucketPath)
}

func (s *s3Implementation) listBuckets() ListBucketsResponse {
	resp := ListBucketsResponse{
		Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
		Owner: Owner{ID: utils.S3OwnerId, DisplayName: "cloudlocal"},
	}

	bkt := make([]Bucket, len(s.buckets))
	for _, bucket := range s.buckets {
		bkt = append(bkt, *bucket)
	}
	resp.Buckets = bkt
	log.Printf("Listing S3 buckets:\n%s\n\n", utils.MarshalIjson(resp))
	return resp
}

func (s *s3Implementation) putObject(bucket, key string, data io.Reader) error {
	// 1. Ensure the directory for the key exists (S3 supports 'folders' via keys)
	filePath := filepath.Join(s.storagePath, bucket, key)
	_ = os.MkdirAll(filepath.Dir(filePath), 0755)

	// 2. Create the file and stream the data into it
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, data)
	log.Printf("Putting object to S3:\nbucket: %s, key: %s\n\n", bucket, key)
	return err
}

func (s *s3Implementation) getObject(bucket, key string) (io.ReadCloser, error) {
	filePath := filepath.Join(s.storagePath, bucket, key)
	log.Printf("Getting object from S3:\nbucket: %s, key: %s\n\n", bucket, key)
	return os.Open(filePath)
}

func (s *s3Implementation) listObjectsV2(bucket, prefix string) (ListObjectsV2Response, error) {
	bucketPath := filepath.Join(s.storagePath, bucket)
	resp := ListObjectsV2Response{
		Xmlns:   "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:    bucket,
		Prefix:  prefix,
		MaxKeys: 1000,
	}

	err := filepath.Walk(bucketPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Convert absolute disk path back to S3 Key
		// Example: /opt/cloudlocal/s3/my-bucket/folder/file.txt -> folder/file.txt
		relPath, _ := filepath.Rel(bucketPath, path)
		key := filepath.ToSlash(relPath)

		// Filter by prefix if provided
		if prefix == "" || strings.HasPrefix(key, prefix) {
			resp.Contents = append(resp.Contents, Object{
				Key:          key,
				LastModified: info.ModTime(),
				ETag:         `"mock-etag"`,
				Size:         info.Size(),
				StorageClass: "STANDARD",
			})
		}
		return nil
	})

	resp.KeyCount = len(resp.Contents)
	log.Printf("Listing objects in bucket:\n%s\n\n", utils.MarshalIjson(resp))
	return resp, err
}

func (s *s3Implementation) deleteObject(bucket, key string) error {
	filePath := filepath.Join(s.storagePath, bucket, key)
	// S3 returns success even if the file doesn't exist
	log.Printf("Deleting object from S3:\nbucket: %s, key: %s\n\n", bucket, key)
	return os.Remove(filePath)
}

func (s *s3Implementation) deleteObjects(bucket string, keys []string) ([]string, error) {
	var deleted []string
	for _, key := range keys {
		filePath := filepath.Join(s.storagePath, bucket, key)
		_ = os.Remove(filePath)
		deleted = append(deleted, key)
	}
	log.Printf("Deleting multiple objects from S3:\nbucket: %s, keys: %v\n\n", bucket, keys)
	return deleted, nil
}

func (s *s3Implementation) initMultipartUpload(bucket, key, uploadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a temporary hidden directory for this specific upload
	uploadDir := filepath.Join(s.storagePath, ".uploads", uploadID)
	err := os.MkdirAll(uploadDir, 0755)
	if err != nil {
		return err
	}
	s.activeUploads[uploadID] = &MultipartUpload{
		UploadID: uploadID,
		Bucket:   bucket,
		Key:      key,
		Parts:    make(map[int]string),
	}
	return nil
}

func (s *s3Implementation) uploadPart(uploadID string, partNum int, data io.Reader) (string, error) {
	// 1. Define where this specific part will live
	uploadDir := filepath.Join(s.storagePath, ".uploads", uploadID)
	partPath := filepath.Join(uploadDir, fmt.Sprintf("part-%d", partNum))

	// 2. Stream the part data to disk
	file, err := os.Create(partPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = io.Copy(file, data)
	if err != nil {
		return "", err
	}

	// 3. Generate a mock ETag (S3 expects this to be quoted)
	etag := fmt.Sprintf("\"etag-part-%d\"", partNum)

	s.mu.Lock()
	if upload, ok := s.activeUploads[uploadID]; ok {
		upload.Parts[partNum] = etag
	}
	s.mu.Unlock()

	return etag, nil
}

func (s *s3Implementation) completeMultipartUpload(bucket, key, uploadID string, body io.Reader) (CompleteMultipartUploadResult, error) {
	// 1. Decode the request to get the part list
	var req CompleteMultipartUploadRequest
	if err := xml.NewDecoder(body).Decode(&req); err != nil {
		return CompleteMultipartUploadResult{}, err
	}

	// 2. Sort parts by PartNumber (S3 requirement)
	sort.Slice(req.Parts, func(i, j int) bool {
		return req.Parts[i].PartNumber < req.Parts[j].PartNumber
	})

	// 3. Create the final destination file
	finalPath := filepath.Join(s.storagePath, bucket, key)
	_ = os.MkdirAll(filepath.Dir(finalPath), 0755)
	finalFile, err := os.Create(finalPath)
	if err != nil {
		return CompleteMultipartUploadResult{}, err
	}
	defer finalFile.Close()

	// 4. Merge each part
	uploadDir := filepath.Join(s.storagePath, ".uploads", uploadID)
	for _, p := range req.Parts {
		partPath := filepath.Join(uploadDir, fmt.Sprintf("part-%d", p.PartNumber))
		partData, err := os.Open(partPath)
		if err != nil {
			continue
		}

		io.Copy(finalFile, partData) // Append to final file
		partData.Close()
	}

	// 5. Cleanup the temporary upload directory
	os.RemoveAll(uploadDir)

	return CompleteMultipartUploadResult{
		Location: fmt.Sprintf("http://localhost:10050/%s/%s", bucket, key),
		Bucket:   bucket,
		Key:      key,
		ETag:     fmt.Sprintf("\"merged-%s\"", uploadID),
	}, nil
}

func (s *s3Implementation) clearBucket(bucketName string) error {
	bucketPath := filepath.Join(s.storagePath, bucketName)

	// Read all files/folders inside the bucket
	files, err := os.ReadDir(bucketPath)
	if err != nil {
		return err
	}

	for _, f := range files {
		os.RemoveAll(filepath.Join(bucketPath, f.Name()))
	}
	return nil
}
