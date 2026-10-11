package analyzer

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// --- XML Strukturen für SOAP Antworten ---

type Envelope struct {
	XMLName xml.Name `xml:"Envelope"`
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

// Netzwerk-Knoten für die Baumstruktur
type NetworkNode struct {
	Name       string
	IP         string
	MAC        string
	DeviceType string // "router", "repeater", "client"
	Active     bool
	Children   []*NetworkNode
}

// Generische SOAP-Anfrage an eine beliebige FRITZ!-IP (Box oder Repeater)
func sendSOAPRequest(deviceIP, service, action string, soapBody string) ([]byte, error) {
	targetURL := fmt.Sprintf("http://%s:49000/upnp/control/%s", deviceIP, service)
	soapAction := fmt.Sprintf("urn:dslforum-org:service:%s:1#%s", service, action)

	envelope := fmt.Sprintf(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
    <s:Body>
        %s
    </s:Body>
</s:Envelope>`, soapBody)

	req, err := http.NewRequest("POST", targetURL, bytes.NewBuffer([]byte(envelope)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "text/xml; charset=\"utf-8\"")
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s"`, soapAction))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// Holt alle Hosts von einem spezifischen FRITZ!-Gerät
func fetchHostsFromDevice(deviceIP string) ([]*NetworkNode, error) {
	bodyCount := `<u:GetHostNumberOfEntries xmlns:u="urn:dslforum-org:service:Hosts:1"/>`
	respData, err := sendSOAPRequest(deviceIP, "Hosts", "GetHostNumberOfEntries", bodyCount)
	if err != nil {
		return nil, err
	}

	var envCount Envelope
	if err := xml.Unmarshal(respData, &envCount); err != nil {
		return nil, err
	}
	count := envCount.Body.GetHostNumberOfEntriesResponse.HostNumberOfEntries

	var hosts []*NetworkNode

	for i := 0; i < count; i++ {
		bodyEntry := fmt.Sprintf(`<u:GetGenericHostEntry xmlns:u="urn:dslforum-org:service:Hosts:1"><NewIndex>%d</NewIndex></u:GetGenericHostEntry>`, i)
		respEntry, err := sendSOAPRequest(deviceIP, "Hosts", "GetGenericHostEntry", bodyEntry)
		if err != nil {
			continue
		}

		var envEntry Envelope
		if err := xml.Unmarshal(respEntry, &envEntry); err != nil {
			continue
		}

		res := envEntry.Body.GetGenericHostEntryResponse
		if res.IPAddress == "" {
			continue
		}

		hosts = append(hosts, &NetworkNode{
			Name:       res.HostName,
			IP:         res.IPAddress,
			MAC:        res.MACAddress,
			DeviceType: "client",
			Active:     res.Active,
		})
	}

	return hosts, nil
}

// Baum grafisch ausgeben
func PrintTree(node *NetworkNode, prefix string, isLast bool) {
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	icon := "💻"
	if node.DeviceType == "router" {
		icon = "🌐 [FRITZ!Box]"
	} else if node.DeviceType == "repeater" {
		icon = "📡 [Repeater]"
	}

	status := ""
	if !node.Active {
		status = " (inaktiv)"
	}

	if node.DeviceType == "client" {
		fmt.Printf("%s%s %s - %s (%s)%s\n", prefix, connector, node.Name, node.IP, node.MAC, status)
	} else {
		fmt.Printf("%s%s %s %s - %s (%s)%s\n", prefix, connector, icon, node.Name, node.IP, node.MAC, status)
	}

	childPrefix := prefix + "│   "
	if isLast {
		childPrefix = prefix + "    "
	}

	for i, child := range node.Children {
		PrintTree(child, childPrefix, i == len(node.Children)-1)
	}
}

func BuildMeshTree(boxIP string, showOnlyActive bool) *NetworkNode {
	fmt.Println("Verbinde mit FRITZ!Box...")

	boxHosts, err := fetchHostsFromDevice(boxIP)
	if err != nil {
		fmt.Printf("Fehler beim Abrufen der FRITZ!Box Hosts: %v\n", err)
		return nil
	}

	root := &NetworkNode{
		Name:       "FRITZ!Box",
		IP:         boxIP,
		MAC:        "Router",
		DeviceType: "router",
		Active:     true,
	}

	// Repeater über Namen identifizieren
	var repeaterIPs []string
	for _, h := range boxHosts {
		if strings.Contains(strings.ToLower(h.Name), "repeater") || strings.Contains(strings.ToLower(h.Name), "powerline") {
			// Wenn Toggle aktiv ist, inaktive Repeater ignorieren
			if showOnlyActive && !h.Active {
				continue
			}
			repeaterIPs = append(repeaterIPs, h.IP)
		}
	}

	seenIPs := make(map[string]bool)
	seenIPs[boxIP] = true

	// Repeater und deren Clients abfragen
	for _, repIP := range repeaterIPs {
		repHosts, err := fetchHostsFromDevice(repIP)
		if err != nil {
			continue
		}

		repName := "FRITZ!Repeater"
		for _, h := range boxHosts {
			if h.IP == repIP {
				repName = h.Name
				seenIPs[h.IP] = true
				break
			}
		}

		repNode := &NetworkNode{
			Name:       repName,
			IP:         repIP,
			DeviceType: "repeater",
			Active:     true,
		}

		for _, client := range repHosts {
			if client.IP == boxIP || client.IP == repIP {
				continue
			}
			// Feature Toggle Filter für Repeater-Clients
			if showOnlyActive && !client.Active {
				continue
			}
			repNode.Children = append(repNode.Children, client)
			seenIPs[client.IP] = true
		}

		// Repeater nur hinzufügen, wenn er aktiv ist oder auch leere angezeigt werden sollen
		root.Children = append(root.Children, repNode)
	}

	// Verbleibende Clients direkt an der FRITZ!Box
	for _, client := range boxHosts {
		if seenIPs[client.IP] {
			continue
		}
		// Feature Toggle Filter für Box-Clients
		if showOnlyActive && !client.Active {
			continue
		}
		root.Children = append(root.Children, client)
		seenIPs[client.IP] = true
	}

	return root
}

func PrintMeshTree(root *NetworkNode, showOnlyActive bool) {
	if root == nil {
		return
	}

	toggleStatus := "Aus"
	if showOnlyActive {
		toggleStatus = "An (nur aktive Geräte)"
	}

	fmt.Printf("\nMesh-Topologie (Feature Toggle 'showOnlyActive': %s):\n", toggleStatus)
	fmt.Println("──────────────────────────────────────────────────────────")
	fmt.Printf("🌐 [FRITZ!Box] %s (%s)\n", root.Name, root.IP)

	for i, child := range root.Children {
		PrintTree(child, "", i == len(root.Children)-1)
	}
}
