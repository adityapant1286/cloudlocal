package health

type ServiceStatus struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Port    int    `json:"port"`
	Type    string `json:"type"` // "Native" or "External"
}
