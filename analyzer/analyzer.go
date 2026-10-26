package analyzer

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"sort"
	"sync"
	"time"
)

// --- PERSISTENZ (JSON SPEICHERN / LADEN) ---
// --- NETZWERK- & WAN-MONITORING ---

func RunScanCycle(ipsToScan []string, routerIP string, networkState map[string]ScanResult, missCounts map[string]int, scanType string, isInitial bool) []string {
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
			IP:           res.IP,
			MAC:          macVal,
			Manufacturer: manufacturerForMAC(macVal),
			Hostname:     res.Hostname,
			Port:         getServiceName(res.Port),
			Latency:      float64(res.Latency.Milliseconds()),
			Jitter:       float64(res.Jitter.Milliseconds()),
			Status:       status,
			ConnectedVia: "Unbekannt",
			MeshIcon:     "❓",
		})
	}

	avgLatency := float64(0)
	if activeHosts > 0 {
		avgLatency = float64((totalLatency / time.Duration(activeHosts)).Milliseconds())
	}

	stats.Lock()
	stats.Devices = webDevices
	if stats.MeshTree != nil {
		enrichDevicesWithMeshInfo(stats.MeshTree)
	}
	stats.History = append(stats.History, HistoryPoint{
		Time:              time.Now().Format("15:04:05"),
		Active:            activeHosts,
		AvgLatency:        avgLatency,
		WanLatency:        stats.CurrentWanLatency,
		WanJitter:         stats.CurrentWanJitter,
		WanProbeSuccesses: stats.WanProbeSuccesses,
	})
	if len(stats.History) > 120 {
		stats.History = stats.History[1:]
	}
	stats.Unlock()

	saveStatsToFile()

	return displayList
}
