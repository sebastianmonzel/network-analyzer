package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// --- KONFIGURATION ---
var commonPorts = []string{
	"80", "443", "8080", "8443", "5000", "631", "9000",
	"21", "22", "53", "135", "139", "445", "548",
	"5353", "7000", "8009", "9100",
}

const statsFileName = "network_stats.json"

// --- DATENSTRUKTUREN FÜR WEBSERVER UND AGENTEN ---

type DeviceState struct {
	IP       string  `json:"ip"`
	MAC      string  `json:"mac"`
	Hostname string  `json:"hostname"`
	Port     string  `json:"port"`
	Latency  float64 `json:"latency_ms"`
	Jitter   float64 `json:"jitter_ms"`
	Status   string  `json:"status"`
}

type HistoryPoint struct {
	Time       string  `json:"time"`
	Active     int     `json:"active"`
	AvgLatency float64 `json:"avg_latency"`
	WanLatency float64 `json:"wan_latency_ms"`
	WanJitter  float64 `json:"wan_jitter_ms"`
}

type AgentState struct {
	Hostname     string  `json:"hostname"`
	IP           string  `json:"ip"`
	Latency      float64 `json:"latency_ms"`
	Jitter       float64 `json:"jitter_ms"`
	WifiRSSI     int     `json:"wifi_rssi_dbm"`
	AccessPoint  string  `json:"access_point"`
	RepeaterName string  `json:"repeater_name"`
	Version      string  `json:"version"`
	LastUpdated  string  `json:"last_updated"`
}

// Zentrale, Thread-sichere Statistik-Datenbank
type NetworkStats struct {
	sync.RWMutex
	Devices           []DeviceState         `json:"devices"`
	History           []HistoryPoint        `json:"history"`
	Agents            map[string]AgentState `json:"agents"`
	CurrentWanLatency float64               `json:"current_wan_latency"`
	CurrentWanJitter  float64               `json:"current_wan_jitter"`
}

var stats = &NetworkStats{
	Agents: make(map[string]AgentState),
}

// --- HILFSFUNKTION: REPEATER-BSSID IN STANDORT ÜBERSETZEN ---
func getRepeaterName(bssid string) string {
	if bssid == "" || bssid == "-" || bssid == "LAN / Unbekannt" || bssid == "00:00:00:00:00:00" {
		return "🔌 LAN / Kabel"
	}

	// TRAGE HIER DIE MAC-ADRESSEN (BSSIDs) DEINER REPEATER / FRITZ!BOX EIN!
	repeaterMap := map[string]string{
		"11:22:33:44:55:66": "🏠 FRITZ!Box (Basis)",
		"aa:bb:cc:dd:ee:ff": "🛋️ Wohnzimmer-Repeater",
		"99:88:77:66:55:44": "🏢 Keller / Büro-Repeater",
	}

	cleanBssid := strings.ToLower(strings.TrimSpace(bssid))

	if name, exists := repeaterMap[cleanBssid]; exists {
		return name
	}

	return "📡 WLAN (" + bssid + ")"
}

// --- HILFSFUNKTION: PORT IN SPRECHENDEN DIENST-NAMEN ÜBERSETZEN ---
func getServiceName(port string) string {
	switch port {
	case "21":
		return "FTP"
	case "22":
		return "SSH"
	case "53":
		return "DNS"
	case "80":
		return "Web (HTTP)"
	case "135":
		return "Windows RPC"
	case "139":
		return "NetBIOS"
	case "443":
		return "Web (HTTPS)"
	case "445":
		return "Windows SMB"
	case "548":
		return "Apple AFP"
	case "631":
		return "Drucker (IPP)"
	case "5000":
		return "UPnP / NAS"
	case "5353":
		return "mDNS (Bonjour)"
	case "7000":
		return "Apple AirPlay"
	case "8009":
		return "Google Chromecast"
	case "8080":
		return "Web (HTTP Alt)"
	case "8443":
		return "Web (HTTPS Alt)"
	case "9000":
		return "Smart TV / Media"
	case "9100":
		return "Netzwerkdrucker"
	case "ICMP":
		return "Ping (ICMP)"
	default:
		return "Port " + port
	}
}

// --- PERSISTENZ (JSON SPEICHERN / LADEN) ---

func saveStatsToFile() {
	stats.RLock()
	data, err := json.MarshalIndent(stats, "", "  ")
	stats.RUnlock()

	if err == nil {
		os.WriteFile(statsFileName, data, 0644)
	}
}

func loadStatsFromFile() {
	file, err := os.ReadFile(statsFileName)
	if err != nil {
		return
	}

	stats.Lock()
	defer stats.Unlock()

	var loaded NetworkStats
	if err := json.Unmarshal(file, &loaded); err == nil {
		if loaded.History != nil {
			stats.History = loaded.History
		}
		if loaded.Devices != nil {
			stats.Devices = loaded.Devices
		}
		if loaded.Agents != nil {
			stats.Agents = loaded.Agents
		}
		fmt.Printf("📂 Vorherige Statistiken aus '%s' geladen (%d Historien-Einträge).\n", statsFileName, len(stats.History))
	}
}

// --- ARP-TABELLE AUSLESEN (MAC-ADRESSEN ERMITTELN) ---

func getMacAddressTable() map[string]string {
	macMap := make(map[string]string)
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.Command("arp", "-a")
	} else {
		cmd = exec.Command("arp", "-n")
	}

	out, err := cmd.Output()
	if err != nil {
		return macMap
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			var ip, mac string
			if runtime.GOOS == "windows" && len(fields) >= 3 {
				ip = strings.Trim(fields[0], "()")
				mac = strings.ToLower(fields[1])
			} else {
				ip = strings.Trim(fields[0], "()")
				mac = strings.ToLower(fields[1])
			}

			parsedIP := net.ParseIP(ip)
			if parsedIP != nil && (strings.Contains(mac, "-") || strings.Contains(mac, ":")) {
				normalizedMac := strings.ReplaceAll(mac, "-", ":")
				macMap[parsedIP.String()] = normalizedMac
			}
		}
	}
	return macMap
}

// --- NETZWERK- & WAN-MONITORING ---

type ScanResult struct {
	IP       string
	MAC      string
	Hostname string
	Port     string
	Latency  time.Duration
	Jitter   time.Duration
}

func getLocalSubnet() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ipv4 := localAddr.IP.To4()
	if ipv4 == nil {
		return "", fmt.Errorf("keine gültige IPv4-Adresse gefunden")
	}
	return fmt.Sprintf("%d.%d.%d.", ipv4[0], ipv4[1], ipv4[2]), nil
}

func resolveHostname(ip string, routerIP string) string {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 500 * time.Millisecond}
			return d.DialContext(ctx, "udp", routerIP+":53")
		},
	}
	names, err := r.LookupAddr(context.Background(), ip)
	if err == nil && len(names) > 0 {
		name := strings.TrimSuffix(names[0], ".")
		name = strings.TrimSuffix(name, ".fritz.box")
		return name
	}
	return "Unbekannt"
}

func systemPing(ip string) (bool, time.Duration) {
	start := time.Now()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "-w", "300", ip)
	} else if runtime.GOOS == "darwin" {
		cmd = exec.Command("ping", "-c", "1", "-W", "300", ip)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
	}
	err := cmd.Run()
	return err == nil, time.Since(start)
}

func measureWanQuality() {
	var latencies []time.Duration
	target := "8.8.8.8:53"

	for i := 0; i < 5; i++ {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", target, 800*time.Millisecond)
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
		if l < minL {
			minL = l
		}
		if l > maxL {
			maxL = l
		}
	}

	avg := float64((total / time.Duration(len(latencies))).Milliseconds())
	jitter := float64((maxL - minL).Milliseconds())

	stats.Lock()
	stats.CurrentWanLatency = avg
	stats.CurrentWanJitter = jitter
	stats.Unlock()
}

func scanHost(ip string, routerIP string, results chan<- ScanResult, wg *sync.WaitGroup) {
	defer wg.Done()

	systemPing(ip)

	for _, port := range commonPorts {
		target := fmt.Sprintf("%s:%s", ip, port)
		start := time.Now()
		conn, err := net.DialTimeout("tcp", target, 400*time.Millisecond)

		if err == nil {
			conn.Close()
			latencies := make([]time.Duration, 0, 5)
			latencies = append(latencies, time.Since(start))

			for i := 0; i < 4; i++ {
				time.Sleep(10 * time.Millisecond)
				s := time.Now()
				c, e := net.DialTimeout("tcp", target, 400*time.Millisecond)
				if e == nil {
					latencies = append(latencies, time.Since(s))
					c.Close()
				}
			}

			var total time.Duration
			minL, maxL := latencies[0], latencies[0]
			for _, l := range latencies {
				total += l
				if l < minL {
					minL = l
				}
				if l > maxL {
					maxL = l
				}
			}

			results <- ScanResult{
				IP:       ip,
				Hostname: resolveHostname(ip, routerIP),
				Port:     port,
				Latency:  total / time.Duration(len(latencies)),
				Jitter:   maxL - minL,
			}
			return
		}
	}

	if alive, latency := systemPing(ip); alive {
		results <- ScanResult{
			IP: ip, Hostname: resolveHostname(ip, routerIP), Port: "ICMP", Latency: latency, Jitter: 0,
		}
	}
}

func fetchAgentData(ip string) {
	client := http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s:8081/data", ip))

	if err == nil {
		defer resp.Body.Close()
		var agentData AgentState

		if err := json.NewDecoder(resp.Body).Decode(&agentData); err == nil {
			agentData.LastUpdated = time.Now().Format("15:04:05")
			agentData.RepeaterName = getRepeaterName(agentData.AccessPoint)

			stats.Lock()
			stats.Agents[ip] = agentData
			stats.Unlock()
		}
	}
}

func runScanCycle(ipsToScan []string, routerIP string, networkState map[string]ScanResult, missCounts map[string]int, scanType string, isInitial bool) []string {
	var wg sync.WaitGroup
	results := make(chan ScanResult, len(ipsToScan))
	currentScan := make(map[string]ScanResult)

	fmt.Printf("\n[%s] Starte %s (%d IPs)...\n", time.Now().Format("15:04:05"), scanType, len(ipsToScan))

	measureWanQuality()

	for _, ip := range ipsToScan {
		wg.Add(1)
		go scanHost(ip, routerIP, results, &wg)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		currentScan[res.IP] = res
	}

	macTable := getMacAddressTable()
	for ip, res := range currentScan {
		if mac, ok := macTable[ip]; ok {
			res.MAC = mac
		} else {
			res.MAC = "-"
		}
		currentScan[ip] = res
	}

	var logMessages []string
	changesFound := false

	changedIPsFrom := make(map[string]bool)
	changedIPsTo := make(map[string]bool)
	lastMacToIP := make(map[string]string)

	if !isInitial {
		for ip, res := range networkState {
			if res.MAC != "" && res.MAC != "-" {
				lastMacToIP[res.MAC] = ip
			}
		}

		for ip, currRes := range currentScan {
			if currRes.MAC != "" && currRes.MAC != "-" {
				if oldIP, exists := lastMacToIP[currRes.MAC]; exists && oldIP != ip {
					msg := fmt.Sprintf("🔄 IP-WECHSEL: Gerät %s (%s) hat neue IP %s (vorher %s)", currRes.Hostname, currRes.MAC, ip, oldIP)
					fmt.Println("  " + msg)
					logMessages = append(logMessages, msg)
					changesFound = true

					changedIPsFrom[oldIP] = true
					changedIPsTo[ip] = true

					delete(networkState, oldIP)
					delete(missCounts, oldIP)
					stats.Lock()
					delete(stats.Agents, oldIP)
					stats.Unlock()
				}
			}
		}
	}

	for ip, res := range currentScan {
		if _, exists := networkState[ip]; !exists && !isInitial {
			if !changedIPsTo[ip] {
				msg := fmt.Sprintf("✅ ONLINE : %-15s (%s) [%s]", ip, res.Hostname, res.MAC)
				fmt.Println("  " + msg)
				logMessages = append(logMessages, msg)
				changesFound = true
			}
		}
		networkState[ip] = res
		missCounts[ip] = 0

		go fetchAgentData(ip)
	}

	for ip, lastRes := range networkState {
		if _, found := currentScan[ip]; !found {
			if changedIPsFrom[ip] {
				continue
			}

			missCounts[ip]++
			if missCounts[ip] >= 5 {
				msg := fmt.Sprintf("❌ OFFLINE: %-15s (%s) [%s] [nach 5 Fehlversuchen]", ip, lastRes.Hostname, lastRes.MAC)
				fmt.Println("  " + msg)
				logMessages = append(logMessages, msg)
				changesFound = true

				delete(networkState, ip)
				delete(missCounts, ip)

				stats.Lock()
				delete(stats.Agents, ip)
				stats.Unlock()
			}
		}
	}

	if changesFound && len(logMessages) > 0 {
		file, err := os.OpenFile("network.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer file.Close()
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			for _, msg := range logMessages {
				fmt.Fprintf(file, "[%s] %s\n", timestamp, msg)
			}
		}
	}

	var displayList []string
	for ip := range networkState {
		displayList = append(displayList, ip)
	}
	sort.Slice(displayList, func(i, j int) bool {
		return bytes.Compare(net.ParseIP(displayList[i]).To4(), net.ParseIP(displayList[j]).To4()) < 0
	})

	var webDevices []DeviceState
	var activeHosts int
	var totalLatency time.Duration

	for _, ip := range displayList {
		res := networkState[ip]
		misses := missCounts[ip]
		status := "Online"

		if misses == 0 {
			activeHosts++
			totalLatency += res.Latency
		} else {
			status = fmt.Sprintf("Fehlt (%d/5)", misses)
		}

		macVal := res.MAC
		if macVal == "" {
			macVal = "-"
		}

		webDevices = append(webDevices, DeviceState{
			IP:       res.IP,
			MAC:      macVal,
			Hostname: res.Hostname,
			Port:     getServiceName(res.Port),
			Latency:  float64(res.Latency.Milliseconds()),
			Jitter:   float64(res.Jitter.Milliseconds()),
			Status:   status,
		})
	}

	avgLatency := float64(0)
	if activeHosts > 0 {
		avgLatency = float64((totalLatency / time.Duration(activeHosts)).Milliseconds())
	}

	stats.Lock()
	stats.Devices = webDevices
	stats.History = append(stats.History, HistoryPoint{
		Time:       time.Now().Format("15:04:05"),
		Active:     activeHosts,
		AvgLatency: avgLatency,
		WanLatency: stats.CurrentWanLatency,
		WanJitter:  stats.CurrentWanJitter,
	})
	if len(stats.History) > 120 {
		stats.History = stats.History[1:]
	}
	stats.Unlock()

	saveStatsToFile()

	return displayList
}

// --- WEBSERVER LOGIK ---

const currentAgentVersion = "1.0"
const agentBinaryFile = "agent_update" // Unter Windows: "agent_update.exe"

func startWebServer() {
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFile(w, r, "static/index.html")
	})

	http.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats.RLock()
		defer stats.RUnlock()
		json.NewEncoder(w).Encode(stats)
	})

	// NEU: Eindeutige Erkennungs-API für Agenten
	http.HandleFunc("/api/is-main-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"server": "network-analyzer-master", "status": "active"}`))
	})

	// NEU: Self-Update Endpunkte
	http.HandleFunc("/api/agent-version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"version": currentAgentVersion})
	})

	http.HandleFunc("/api/agent-binary", func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(agentBinaryFile); os.IsNotExist(err) {
			http.Error(w, "Update-Datei nicht gefunden", http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, agentBinaryFile)
	})

	fmt.Println("\n=======================================================")
	fmt.Println("🚀 WEBDASHBOARD GESTARTET: http://localhost:8080")
	fmt.Println("=======================================================\n")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func main() {
	loadStatsFromFile()

	subnet, err := getLocalSubnet()
	if err != nil {
		log.Fatalf("Fehler bei der Ermittlung des Subnetzes: %v\n", err)
	}

	routerIP := subnet + "1"

	fmt.Printf("Lokales Subnetz %s0/24 erkannt.\n", subnet)
	fmt.Printf("Nutze Router %s als DNS-Server.\n", routerIP)

	var allIPs []string
	for i := 1; i <= 254; i++ {
		allIPs = append(allIPs, fmt.Sprintf("%s%d", subnet, i))
	}

	networkState := make(map[string]ScanResult)
	missCounts := make(map[string]int)

	go startWebServer()

	knownIPs := runScanCycle(allIPs, routerIP, networkState, missCounts, "INITIALER KOMPLETT-SCAN", true)

	fmt.Printf("\n>>> Überwachung gestartet.\n")
	fmt.Println(">>> - Schnellprüfung:  alle 30 Sekunden")
	fmt.Println(">>> - Komplett-Scan:   alle 5 Minuten")
	fmt.Println(">>> - Dynamische Agenten-Erkennung & Updates: Aktiv")
	fmt.Println(">>> (Abbruch mit Strg+C)")

	scanInterval := 30 * time.Second
	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()

	ticks := 0
	for range ticker.C {
		ticks++
		if ticks >= 10 {
			knownIPs = runScanCycle(allIPs, routerIP, networkState, missCounts, "5-MINUTEN KOMPLETT-SCAN", false)
			ticks = 0
		} else {
			knownIPs = runScanCycle(knownIPs, routerIP, networkState, missCounts, "SCHNELL-PRÜFUNG", false)
		}
	}
}

// --- HTML / JS FRONTEND ---
