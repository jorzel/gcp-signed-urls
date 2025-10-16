package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

// GenerateUploadPolicy generates a signed POST policy for a specific user/job folder.
func GenerateUploadPolicy(
	ctx context.Context, client *storage.Client, bucket, username, jobID string, expireMinutes int,
) (*storage.PostPolicyV4, string, error) {
	prefix := fmt.Sprintf("%s/%s/", username, jobID)
	objectKey := prefix + "${filename}"

	opts := &storage.PostPolicyV4Options{
		Expires: time.Now().Add(time.Duration(expireMinutes) * time.Minute),
		Fields: &storage.PolicyV4Fields{
			ContentEncoding: "gzip",
		},
		Conditions: []storage.PostPolicyV4Condition{
			storage.ConditionStartsWith("$key", prefix),
			storage.ConditionStartsWith("$Content-Encoding", ""),
		},
	}

	policy, err := client.Bucket(bucket).GenerateSignedPostPolicyV4(objectKey, opts)
	if err != nil {
		return nil, "", fmt.Errorf("GenerateSignedPostPolicyV4: %w", err)
	}
	return policy, prefix, nil
}

// CreateGzippedFile creates a text file in memory and gzips it to disk.
func CreateGzippedFile(filePath, content string) error {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(content)); err != nil {
		return fmt.Errorf("gzip write: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("gzip close: %w", err)
	}

	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write gz file: %w", err)
	}
	return nil
}

// UploadFileWithPolicy uploads a local file using the generated policy.
func UploadFileWithPolicy(policy *storage.PostPolicyV4, localFile, objectKey string) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add policy fields
	for k, v := range policy.Fields {
		if err := writer.WriteField(k, v); err != nil {
			return fmt.Errorf("write field %s: %w", k, err)
		}
	}

	// Add file
	fileWriter, err := writer.CreateFormFile("file", localFile)
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}

	file, err := os.Open(localFile)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(fileWriter, file); err != nil {
		return fmt.Errorf("copy file: %w", err)
	}

	writer.Close()

	// POST request
	req, err := http.NewRequest("POST", policy.URL, &buf)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed: status %s, body: %s", resp.Status, string(body))
	}

	fmt.Printf("✅ Upload succeeded! Object key: %s\n", objectKey)
	return nil
}

func main() {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("storage.NewClient: %v", err)
	}
	defer client.Close()

	bucket := os.Getenv("GCS_BUCKET")
	username := "alice"
	jobID := uuid.New().String()
	localFile := "/tmp/test.gz"
	objectName := "test.gz" // file name in the bucket

	// 1️⃣ Generate signed POST policy
	policy, prefix, err := GenerateUploadPolicy(ctx, client, bucket, username, jobID, 15) // expires 15min
	if err != nil {
		log.Fatalf("GenerateUploadPolicy: %v", err)
	}

	// Optional: print policy JSON
	policyJSON, _ := json.MarshalIndent(policy, "", "  ")
	fmt.Println("Generated POST policy:", string(policyJSON))

	// 2️⃣ Create gzipped file
	content := "Hello world! This is a test file for GCS upload.\nLine 2 of the file."
	if err := CreateGzippedFile(localFile, content); err != nil {
		log.Fatalf("CreateGzippedFile: %v", err)
	}

	// 3️⃣ Upload file using policy
	objectKey := fmt.Sprintf("%s%s", prefix, objectName)
	if err := UploadFileWithPolicy(policy, localFile, objectKey); err != nil {
		log.Fatalf("UploadFileWithPolicy: %v", err)
	}
}
