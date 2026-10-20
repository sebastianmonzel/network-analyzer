package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type ScanResult struct {
	IP       string
	MAC      string
	Hostname string
	Port     string
	Latency  time.Duration
	Jitter   time.Duration
}

func GetLocalSubnet() (string, error) {
	return getLocalSubnet()
}

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
			} else if runtime.GOOS != "windows" {
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
