package analyzer

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
)

const currentAgentVersion = "1.0"
const agentBinaryFile = "agent_update" // Unter Windows: "agent_update.exe"

func StartWebServer() {
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

	http.HandleFunc("/api/caller", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Methode nicht erlaubt", http.StatusMethodNotAllowed)
			return
		}

		ip, err := callerIP(r.RemoteAddr)
		if err != nil {
			http.Error(w, "Aufrufer-IP konnte nicht ermittelt werden", http.StatusBadRequest)
			return
		}

		result := struct {
			IP       string         `json:"ip"`
			Device   *DeviceState   `json:"device"`
			Agent    *AgentState    `json:"agent"`
			MeshPath []*NetworkNode `json:"mesh_path"`
		}{IP: ip}

		stats.RLock()
		result.MeshPath = findMeshPath(stats.MeshTree, ip)
		for i := range stats.Devices {
			if sameIP(stats.Devices[i].IP, ip) {
				device := stats.Devices[i]
				result.Device = &device
				break
			}
		}
		for _, agent := range stats.Agents {
			if sameIP(agent.IP, ip) {
				agentCopy := agent
				result.Agent = &agentCopy
				break
			}
		}
		stats.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			log.Printf("Aufruferdaten konnten nicht ausgegeben werden: %v", err)
		}
	})

	http.HandleFunc("/api/mesh", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats.RLock()
		meshTree := stats.MeshTree
		stats.RUnlock()
		if meshTree == nil {
			w.Write([]byte(`{"mesh_tree": null}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"mesh_tree": meshTree})
	})

	http.HandleFunc("/api/is-main-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"server": "network-analyzer-master", "status": "active"}`))
	})

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

func callerIP(remoteAddr string) (string, error) {
	return callerIPWithLocalAddress(remoteAddr, localNetworkIP)
}

func callerIPWithLocalAddress(remoteAddr string, getLocalIP func() (string, error)) (string, error) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	host = strings.Trim(host, "[]")

	ip, err := netip.ParseAddr(host)
	if err != nil {
		return "", fmt.Errorf("ungültige Aufrufer-IP %q: %w", host, err)
	}
	ip = ip.WithZone("").Unmap()
	if ip.IsLoopback() {
		localIP, err := getLocalIP()
		if err != nil {
			return "", fmt.Errorf("lokale Netzwerk-IP konnte für Loopback-Aufruf nicht ermittelt werden: %w", err)
		}
		ip, err = netip.ParseAddr(localIP)
		if err != nil {
			return "", fmt.Errorf("ungültige lokale Netzwerk-IP %q", localIP)
		}
		ip = ip.WithZone("").Unmap()
		if ip.IsLoopback() {
			return "", fmt.Errorf("ungültige lokale Netzwerk-IP %q", localIP)
		}
	}
	return ip.String(), nil
}

func localNetworkIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("Standard-Netzwerkroute konnte nicht ermittelt werden: %w", err)
	}
	defer conn.Close()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || localAddr.IP.IsUnspecified() || localAddr.IP.IsLoopback() {
		return "", fmt.Errorf("keine gültige lokale Netzwerk-IP gefunden")
	}
	return localAddr.IP.String(), nil
}

func sameIP(left, right string) bool {
	leftIP, err := netip.ParseAddr(strings.Trim(left, "[]"))
	if err != nil {
		return false
	}
	rightIP, err := netip.ParseAddr(strings.Trim(right, "[]"))
	if err != nil {
		return false
	}
	return leftIP.WithZone("").Unmap() == rightIP.WithZone("").Unmap()
}

func findMeshPath(root *NetworkNode, targetIP string) []*NetworkNode {
	if root == nil {
		return nil
	}

	if sameIP(root.IP, targetIP) {
		return []*NetworkNode{{Name: root.Name, IP: root.IP, MAC: root.MAC, DeviceType: root.DeviceType, Active: root.Active}}
	}

	for _, child := range root.Children {
		path := findMeshPath(child, targetIP)
		if path != nil {
			node := &NetworkNode{
				Name:       root.Name,
				IP:         root.IP,
				MAC:        root.MAC,
				DeviceType: root.DeviceType,
				Active:     root.Active,
			}
			return append([]*NetworkNode{node}, path...)
		}
	}
	return nil
}
