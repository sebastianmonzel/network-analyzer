package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const listenPort = ":8081"
const checkTarget = "8.8.8.8:53"
const agentVersion = "1.0"

var cachedServerURL = "http://localhost:8080"
var serverMutex sync.RWMutex

// Dynamische Suche nach dem Hauptserver im lokalen Subnetz
func discoverMainServer() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "http://localhost:8080"
	}
	defer conn.Close()

	localIP := conn.LocalAddr().(*net.UDPAddr).IP.String()
	parts := strings.Split(localIP, ".")
	if len(parts) != 4 {
		return "http://localhost:8080"
	}

	basePrefix := fmt.Sprintf("%s.%s.%s.", parts[0], parts[1], parts[2])
	foundURL := make(chan string, 1)
	var wg sync.WaitGroup
	client := http.Client{Timeout: 400 * time.Millisecond}

	// Subnetz von .1 bis .254 parallel nach dem Hauptserver abfragen
	for i := 1; i < 255; i++ {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			url := fmt.Sprintf("http://%s:8080/api/is-main-server", ip)
			resp, err := client.Get(url)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					select {
					case foundURL <- fmt.Sprintf("http://%s:8080", ip):
					default:
					}
				}
			}
		}(fmt.Sprintf("%s%d", basePrefix, i))
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case url := <-foundURL:
		fmt.Printf("🎯 Hauptserver dynamisch gefunden: %s\n", url)
		return url
	case <-done:
		return "http://localhost:8080"
	case <-time.After(2 * time.Second):
		return "http://localhost:8080"
	}
}

func getServerBaseURL() string {
	serverMutex.RLock()
	url := cachedServerURL
	serverMutex.RUnlock()
	return url
}

type AgentPayload struct {
	Hostname     string  `json:"hostname"`
	IP           string  `json:"ip"`
	Latency      float64 `json:"latency_ms"`
	Jitter       float64 `json:"jitter_ms"`
	WifiRSSI     int     `json:"wifi_rssi_dbm"`
	AccessPoint  string  `json:"access_point"`
	RepeaterName string  `json:"repeater_name"`
	Version      string  `json:"version"`
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
	bssid := "LAN / Unbekannt"

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
						bssid = strings.TrimSpace(strings.Join(parts[1:], ":"))
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
			bssid = strings.TrimSpace(string(out))
		}
	}
	return rssi, bssid
}

// --- SELF-UPDATE LOGIK ---
func checkForUpdate() {
	serverURL := getServerBaseURL()
	resp, err := http.Get(serverURL + "/api/agent-version")
	if err != nil {
		// Wenn Verbindung fehlschlägt, Serveradresse neu suchen
		newURL := discoverMainServer()
		serverMutex.Lock()
		cachedServerURL = newURL
		serverMutex.Unlock()
		return
	}
	defer resp.Body.Close()

	var result struct {
		Version string `json:"version"`
	}
	if json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	if result.Version != "" && result.Version != agentVersion {
		fmt.Printf("🔄 Neue Agenten-Version %s entdeckt (aktuell: %s). Starte Update...\n", result.Version, agentVersion)

		binResp, err := http.Get(serverURL + "/api/agent-binary")
		if err != nil {
			return
		}
		defer binResp.Body.Close()

		ex, err := os.Executable()
		if err != nil {
			return
		}

		tmpFile := ex + ".new"
		out, err := os.Create(tmpFile)
		if err != nil {
			return
		}
		io.Copy(out, binResp.Body)
		out.Close()

		os.Chmod(tmpFile, 0755)

		oldFile := ex + ".old"
		os.Remove(oldFile)

		if err := os.Rename(ex, oldFile); err != nil {
			return
		}

		if err := os.Rename(tmpFile, ex); err != nil {
			os.Rename(oldFile, ex)
			return
		}

		fmt.Println("✅ Update erfolgreich! Starte Agent neu...")
		cmd := exec.Command(ex, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Start()
		os.Exit(0)
	}
}

func measureQuality() {
	var latencies []time.Duration
	for i := 0; i < 5; i++ {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", checkTarget, 500*time.Millisecond)
		if err == nil {
			latencies = append(latencies, time.Since(start))
			conn.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}

	if len(latencies) == 0 {
		return
	}

	var total time.Duration
	minL, maxL := latencies[0], latencies[0]
	for _, l := range latencies {
		total += l
		if l < minL { minL = l }
		if l > maxL { maxL = l }
	}

	avg := float64((total / time.Duration(len(latencies))).Milliseconds())
	jitter := float64((maxL - minL).Milliseconds())
	rssi, bssid := getWifiInfo()

	dataMutex.Lock()
	currentData.Latency = avg
	currentData.Jitter = jitter
	currentData.WifiRSSI = rssi
	currentData.AccessPoint = bssid
	dataMutex.Unlock()
}

func main() {
	hostname, _ := os.Hostname()
	myIP := getLocalIP()

	dataMutex.Lock()
	currentData = AgentPayload{
		Hostname:    hostname,
		IP:          myIP,
		AccessPoint: "LAN / Unbekannt",
		Version:     agentVersion,
	}
	dataMutex.Unlock()

	// Hauptserver beim Start im Netzwerk suchen
	go func() {
		serverURL := discoverMainServer()
		serverMutex.Lock()
		cachedServerURL = serverURL
		serverMutex.Unlock()
	}()

	// 1. Hintergrund-Job: Qualitätsmessung alle 30 Sekunden
	go func() {
		measureQuality()
		for {
			time.Sleep(30 * time.Second)
			measureQuality()
		}
	}()

	// 2. Hintergrund-Job: Update-Prüfung alle 10 Minuten
	go func() {
		time.Sleep(10 * time.Second) // Warten bis Server-Suche durch ist
		checkForUpdate()
		for {
			time.Sleep(10 * time.Minute)
			checkForUpdate()
		}
	}()

	// 3. Webserver des Agenten
	http.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		dataMutex.RLock()
		defer dataMutex.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(currentData)
	})

	fmt.Printf("🚀 Agent v%s läuft auf %s%s\n", agentVersion, myIP, listenPort)
	http.ListenAndServe(listenPort, nil)
}