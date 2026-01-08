package health

const SERVICE = "health-service"

type ServiceStatus struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Port    int    `json:"port"`
	Type    string `json:"type"` // "Native" or "External"
}
