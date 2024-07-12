package model

type Instance struct {
	ZoneID     string `json:"zone_id"`
	SiteID     string `json:"site_id"`
	ServerIP   string `json:"server_ip"`
	InstanceID string `json:"instance_id"`
	PodName    string `json:"pod_name"`
	Port       int    `json:"port"`
	IsElastic  int    `json:"is_elastic"`
	Status     string `json:"status"`
	DeviceId   string `json:"device_id"`
}

type Record struct {
	ZoneID    string `json:"zone_id"`
	SiteID    string `json:"site_id"`
	Date      string `json:"date"`
	Instances int    `json:"instance_id"`
}
