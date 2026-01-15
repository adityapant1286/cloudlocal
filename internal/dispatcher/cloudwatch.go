package dispatcher

import (
	"cloudlocal/internal/utils"
	"net/http"
)

func (d *Dispatcher) HandleCloudWatchAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/dashboard/api/logs/groups":
		groupNames := d.CwSvc.ListGroupNames()
		w.WriteHeader(http.StatusOK)
		utils.RespondJSON(w, map[string]any{"logGroups": groupNames})

	case "/dashboard/api/logs/streams":
		group := r.URL.Query().Get("group")
		logStreams := d.CwSvc.ListLogStreamsNames(group)
		w.WriteHeader(http.StatusOK)
		utils.RespondJSON(w, map[string]any{"logStreams": logStreams})
	case "/dashboard/api/logs/events":
		group := r.URL.Query().Get("group")
		stream := r.URL.Query().Get("stream")
		events := d.CwSvc.ListEvents(group, stream)
		w.WriteHeader(http.StatusOK)
		utils.RespondJSON(w, map[string]any{"events": events})
	}

}
