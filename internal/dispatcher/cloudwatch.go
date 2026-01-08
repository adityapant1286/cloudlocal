package dispatcher

import (
	"cloudlocal/internal/utils"
	"net/http"
)

func (d *Dispatcher) HandleCloudWatchAdmin(w http.ResponseWriter, r *http.Request) {
	//action := ""
	//var payload []byte
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/dashboard/api/logs/groups":
		//action = "DescribeLogGroups"
		groupNames := d.CwSvc.ListGroupNames()
		w.WriteHeader(http.StatusOK)
		utils.RespondJSON(w, map[string]any{"logGroups": groupNames})

	case "/dashboard/api/logs/streams":
		group := r.URL.Query().Get("group")
		//action = "DescribeLogStreams"
		logStreams := d.CwSvc.ListLogStreamsNames(group)
		w.WriteHeader(http.StatusOK)
		utils.RespondJSON(w, map[string]any{"logStreams": logStreams})
		//payload = []byte(fmt.Sprintf(`{"logGroupName": "%s", "orderBy": "LastEventTime", "descending": true}`, group))
	case "/dashboard/api/logs/events":
		group := r.URL.Query().Get("group")
		stream := r.URL.Query().Get("stream")
		//action = "GetLogEvents"
		events := d.CwSvc.ListEvents(group, stream)
		w.WriteHeader(http.StatusOK)
		utils.RespondJSON(w, map[string]any{"events": events})
		//payload = []byte(fmt.Sprintf(`{"logGroupName": "%s", "logStreamName": "%s", "limit": 100}`, group, stream))
	}

	//if action != "" {
	//	d.ProxyToCloudWatch(w, r, action, payload)
	//}
}

/*
func (d *Dispatcher) ProxyToCloudWatch(w http.ResponseWriter, r *http.Request, action string, payload []byte) {
	req, _ := http.NewRequest("POST", utils.CloudLocalUrl, bytes.NewBuffer(payload))

	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "Logs_20140530."+action)
	req.Header.Set("x-amz-date", "20260101T000000Z")
	req.Header.Set("Authorization", utils.ApiAuthHeader("logs"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "KMS Proxy Error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "KMS Proxy Copy Error: "+err.Error(), 500)
		return
	}
}
*/
