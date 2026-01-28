package health

import (
	"cloudlocal/internal/cloudwatch"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ProbeInternalServices(cw cloudwatch.CwService, enabledServices string) []ServiceStatus {
	// Simple check if a service name exists in the env string
	contains := func(s, substr string) bool {
		return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
	}

	var stats []ServiceStatus

	// 1. Check DynamoDB (External Process on 10051)
	if contains(enabledServices, "dynamodb") {
		conn, err := net.DialTimeout("tcp", "localhost:10051", 500*time.Millisecond)
		isAlive := err == nil
		if isAlive {
			err := conn.Close()
			if err != nil {
				cw.Error(SERVICE, "DynamoDb", fmt.Sprintf("Failed to close DynamoDb connection: %v", err.Error()))
				return nil
			}
		}

		stats = append(stats, ServiceStatus{
			Id: "dynamodb", Name: "DynamoDB", Healthy: isAlive, Port: 10050, Type: "External",
		})
	}

	// 2. Check Native Mocks (Always "Healthy" if code is running)
	if contains(enabledServices, "kms") {
		stats = append(stats, ServiceStatus{
			Id: "kms", Name: "KMS", Healthy: true, Port: 10050, Type: "Native",
		})
	}
	if contains(enabledServices, "secretsmanager") {
		stats = append(stats, ServiceStatus{
			Id: "secretsmanager", Name: "Secrets Manager", Healthy: true, Port: 10050, Type: "Native",
		})
	}
	if contains(enabledServices, "s3") {
		stats = append(stats, ServiceStatus{
			Id: "s3", Name: "S3", Healthy: true, Port: 10050, Type: "Native",
		})
	}
	if contains(enabledServices, "sqs") {
		stats = append(stats, ServiceStatus{
			Id: "sqs", Name: "SQS", Healthy: true, Port: 10050, Type: "Native",
		})
	}
	if contains(enabledServices, "sns") {
		stats = append(stats, ServiceStatus{
			Id: "sns", Name: "SNS", Healthy: true, Port: 10050, Type: "Native",
		})
	}

	return stats
}

func GetStorageStats(volumeDir string) map[string]string {
	services := []string{"s3", "dynamodb", "sqs", "sns", "kms", "secretsmanager"}
	stats := make(map[string]string)

	for _, svc := range services {
		path := filepath.Join(volumeDir, svc)
		size, err := dirSize(path)
		if err != nil {
			stats[svc] = "0 B"
			continue
		}
		stats[svc] = formatBytes(size)
	}
	return stats
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
