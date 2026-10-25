package main

import (
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type AgentPayload struct {
	Hostname          string  `json:"hostname"`
	IP                string  `json:"ip"`
	Latency           float64 `json:"latency_ms"`
	Jitter            float64 `json:"jitter_ms"`
	WifiRSSI          int     `json:"wifi_rssi_dbm"`
	AccessPoint       string  `json:"access_point"`
	RepeaterName      string  `json:"repeater_name"`
	Version           string  `json:"version"`
	LastCheckedAt     string  `json:"last_checked_at,omitempty"`
	LastMeasurementAt string  `json:"last_measurement_at,omitempty"`
	ProbeAttempts     int     `json:"probe_attempts"`
	ProbeSuccesses    int     `json:"probe_successes"`
}

var (
	currentData AgentPayload
	dataMutex   sync.RWMutex
)

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func getWifiInfo() (int, string) {
	rssi := 0
	// setzt unter windows die WLAN-Access-Points;
	// unter Linux wird stattdessen die SSID (WLAN-Name) ausgelesen.
	accessPoint := "LAN / Unbekannt"

	if runtime.GOOS == "windows" {
		cmd := exec.Command("netsh", "wlan", "show", "interfaces")
		out, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.Contains(line, "Signal") {
					parts := strings.Split(line, ":")
					if len(parts) == 2 {
						valStr := strings.TrimSpace(strings.ReplaceAll(parts[1], "%", ""))
						if val, err := strconv.Atoi(valStr); err == nil {
							rssi = (val / 2) - 100
						}
					}
				}
				if strings.Contains(line, "BSSID") {
					parts := strings.Split(line, ":")
					if len(parts) >= 2 {
						accessPoint = strings.TrimSpace(strings.Join(parts[1:], ":"))
					}
				}
			}
		}
	} else if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/net/wireless")
		if err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.Contains(line, ":") {
					fields := strings.Fields(line)
					if len(fields) >= 4 {
						levelStr := strings.TrimRight(fields[3], ".")
						if val, err := strconv.Atoi(levelStr); err == nil {
							rssi = val
						}
					}
				}
			}
		}
		cmd := exec.Command("iwgetid", "-a", "-r")
		out, err := cmd.Output()
		if err == nil {
			accessPoint = strings.TrimSpace(string(out))
		}
	}
	return rssi, accessPoint
}
