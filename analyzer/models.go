package analyzer

import "sync"

type DeviceState struct {
	IP           string  `json:"ip"`
	MAC          string  `json:"mac"`
	Hostname     string  `json:"hostname"`
	Port         string  `json:"port"`
	Latency      float64 `json:"latency_ms"`
	Jitter       float64 `json:"jitter_ms"`
	Status       string  `json:"status"`
	ConnectedVia string  `json:"connected_via"`
	MeshIcon     string  `json:"mesh_icon"`
}

type HistoryPoint struct {
	Time              string  `json:"time"`
	Active            int     `json:"active"`
	AvgLatency        float64 `json:"avg_latency"`
	WanLatency        float64 `json:"wan_latency_ms"`
	WanJitter         float64 `json:"wan_jitter_ms"`
	WanProbeSuccesses int     `json:"wan_probe_successes"`
}

type AgentState struct {
	Hostname          string  `json:"hostname"`
	IP                string  `json:"ip"`
	Latency           float64 `json:"latency_ms"`
	Jitter            float64 `json:"jitter_ms"`
	WifiRSSI          int     `json:"wifi_rssi_dbm"`
	AccessPoint       string  `json:"access_point"`
	RepeaterName      string  `json:"repeater_name"`
	Version           string  `json:"version"`
	LastUpdated       string  `json:"last_updated"`
	LastCheckedAt     string  `json:"last_checked_at,omitempty"`
	LastMeasurementAt string  `json:"last_measurement_at,omitempty"`
	ProbeAttempts     int     `json:"probe_attempts"`
	ProbeSuccesses    int     `json:"probe_successes"`
}

type NetworkNode struct {
	Name       string         `json:"name"`
	IP         string         `json:"ip"`
	MAC        string         `json:"mac"`
	DeviceType string         `json:"device_type"`
	Active     bool           `json:"active"`
	Children   []*NetworkNode `json:"children,omitempty"`
}

type Envelope struct {
	XMLName struct{} `xml:"Envelope"`
	Body    Body     `xml:"Body"`
}

type Body struct {
	GetHostNumberOfEntriesResponse struct {
		HostNumberOfEntries int `xml:"NewHostNumberOfEntries"`
	} `xml:"GetHostNumberOfEntriesResponse"`

	GetGenericHostEntryResponse struct {
		HostName   string `xml:"NewHostName"`
		IPAddress  string `xml:"NewIPAddress"`
		MACAddress string `xml:"NewMACAddress"`
		Active     bool   `xml:"NewActive"`
	} `xml:"GetGenericHostEntryResponse"`
}

type NetworkStats struct {
	sync.RWMutex
	Devices              []DeviceState         `json:"devices"`
	History              []HistoryPoint        `json:"history"`
	Agents               map[string]AgentState `json:"agents"`
	MeshTree             *NetworkNode          `json:"mesh_tree,omitempty"`
	CurrentWanLatency    float64               `json:"current_wan_latency"`
	CurrentWanJitter     float64               `json:"current_wan_jitter"`
	WanLastCheckedAt     string                `json:"wan_last_checked_at,omitempty"`
	WanLastMeasurementAt string                `json:"wan_last_measurement_at,omitempty"`
	WanProbeAttempts     int                   `json:"wan_probe_attempts"`
	WanProbeSuccesses    int                   `json:"wan_probe_successes"`
	LastMeshUpdate       string                `json:"last_mesh_update"`
}

var stats = &NetworkStats{
	Agents: make(map[string]AgentState),
}

func GetStats() *NetworkStats {
	stats.RLock()
	defer stats.RUnlock()
	return stats
}
