package servicediscovery

type ServiceStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Port    int    `json:"port,omitempty"`
	Version string `json:"version,omitempty"`
}
