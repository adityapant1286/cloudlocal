package dispatcher

import (
	"fmt"
	"io"
	"net/http"
)

func (d *Dispatcher) HandleDynamoAdmin(w http.ResponseWriter, r *http.Request) {

	// 1. List Tables
	if r.URL.Path == "/dashboard/api/dynamo/tables" {
		// DynamoDB ListTables call
		d.ProxyToDynamo(w, r, "ListTables", []byte("{}"))
		return
	}

	// 2. Scan Table (with Pagination)
	if r.URL.Path == "/dashboard/api/dynamo/scan" {
		body, _ := io.ReadAll(r.Body)
		d.ProxyToDynamo(w, r, "Scan", body)
		return
	}

	if r.URL.Path == "/dashboard/api/dynamo/describe" {
		tableName := r.URL.Query().Get("table")
		payload := []byte(fmt.Sprintf(`{"TableName": "%s"}`, tableName))
		d.ProxyToDynamo(w, r, "DescribeTable", payload)
		return
	}

	if r.URL.Path == "/dashboard/api/dynamo/put_item" {
		body, _ := io.ReadAll(r.Body)
		// We proxy directly to DynamoDB Local's PutItem action
		d.ProxyToDynamo(w, r, "PutItem", body)
		return
	}

	if r.URL.Path == "/dashboard/api/dynamo/delete" {
		body, _ := io.ReadAll(r.Body)
		// We proxy directly to DynamoDB Local's PutItem action
		d.ProxyToDynamo(w, r, "DeleteItem", body)
		return
	}
}
