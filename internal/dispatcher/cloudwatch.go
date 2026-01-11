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

}
