package analyzer

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

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
			Name:         res.HostName,
			IP:           res.IPAddress,
			MAC:          res.MACAddress,
			Manufacturer: manufacturerForMAC(res.MACAddress),
			DeviceType:   "client",
			Active:       res.Active,
		})
	}

	return hosts, nil
}

func meshInfrastructureType(name string) (string, bool) {
	name = strings.ToLower(name)
	if strings.Contains(name, "powerline") || strings.Contains(name, "power-line") {
		return "powerline", true
	}
	if strings.Contains(name, "repeater") {
		return "repeater", true
	}
	return "", false
}

func appendMeshClients(parent *NetworkNode, hosts []*NetworkNode, seenIPs map[string]bool, showOnlyActive bool) {
	for _, client := range hosts {
		if seenIPs[client.IP] || (showOnlyActive && !client.Active) {
			continue
		}
		parent.Children = append(parent.Children, client)
		seenIPs[client.IP] = true
	}
}

func buildMeshTree(boxIP string, showOnlyActive bool) *NetworkNode {
	fmt.Println("[Mesh] Verbinde mit FRITZ!Box...")

	boxHosts, err := fetchHostsFromDevice(boxIP)
	if err != nil {
		fmt.Printf("[Mesh] Fehler beim Abrufen der FRITZ!Box Hosts: %v\n", err)
		return nil
	}

	root := &NetworkNode{
		Name:       "FRITZ!Box",
		IP:         boxIP,
		MAC:        "Router",
		DeviceType: "router",
		Active:     true,
	}

	var infrastructureNodes []*NetworkNode
	seenIPs := make(map[string]bool)
	seenIPs[boxIP] = true
	for _, h := range boxHosts {
		deviceType, isInfrastructure := meshInfrastructureType(h.Name)
		if !isInfrastructure || (showOnlyActive && !h.Active) {
			continue
		}
		node := *h
		node.DeviceType = deviceType
		node.Manufacturer = manufacturerForMAC(node.MAC)
		node.Children = nil
		infrastructureNodes = append(infrastructureNodes, &node)
		seenIPs[h.IP] = true
	}

	for _, infrastructureNode := range infrastructureNodes {
		repHosts, err := fetchHostsFromDevice(infrastructureNode.IP)
		if err != nil {
			fmt.Printf("[Mesh] Geräteliste von %s (%s) konnte nicht abgerufen werden: %v\n", infrastructureNode.Name, infrastructureNode.IP, err)
		}
		appendMeshClients(infrastructureNode, repHosts, seenIPs, showOnlyActive)
		root.Children = append(root.Children, infrastructureNode)
	}

	appendMeshClients(root, boxHosts, seenIPs, showOnlyActive)

	return root
}

func updateMeshTree(boxIP string) {
	meshTree := buildMeshTree(boxIP, true)
	if meshTree != nil {
		stats.Lock()
		stats.MeshTree = meshTree
		stats.LastMeshUpdate = time.Now().Format("15:04:05")
		enrichDevicesWithMeshInfo(meshTree)
		stats.Unlock()
		fmt.Println("[Mesh] ✅ Mesh-Topologie aktualisiert")
	}
}

func buildIPToParentMap(node *NetworkNode, parentName string, parentIcon string, ipMap map[string]map[string]string) {
	if node == nil {
		return
	}

	icon := ""
	switch node.DeviceType {
	case "router":
		icon = "🌐"
	case "repeater":
		icon = "📡"
	case "powerline":
		icon = "🔌"
	default:
		icon = "💻"
	}

	for _, child := range node.Children {
		if child.DeviceType == "client" {
			ipMap[child.IP] = map[string]string{
				"parent_name": node.Name,
				"parent_icon": icon,
			}
		} else if child.DeviceType == "repeater" || child.DeviceType == "powerline" {
			ipMap[child.IP] = map[string]string{
				"parent_name": node.Name,
				"parent_icon": icon,
			}
			childIcon := "📡"
			if child.DeviceType == "powerline" {
				childIcon = "🔌"
			}
			buildIPToParentMap(child, child.Name, childIcon, ipMap)
		}
	}
}

func enrichDevicesWithMeshInfo(meshTree *NetworkNode) {
	if meshTree == nil {
		return
	}

	ipMap := make(map[string]map[string]string)
	buildIPToParentMap(meshTree, meshTree.Name, "🌐", ipMap)

	for i := range stats.Devices {
		if parentInfo, exists := ipMap[stats.Devices[i].IP]; exists {
			stats.Devices[i].ConnectedVia = parentInfo["parent_name"]
			stats.Devices[i].MeshIcon = parentInfo["parent_icon"]
		} else {
			stats.Devices[i].ConnectedVia = "Unbekannt"
			stats.Devices[i].MeshIcon = "❓"
		}
	}
}

func UpdateMeshTopology(boxIP string) {
	updateMeshTree(boxIP)
}

func PrintMeshTree(root *NetworkNode) {
	if root == nil {
		fmt.Println("[Mesh] Keine Topologie erkannt")
		return
	}

	printMeshNode(root, "", true, true)
}

func printMeshNode(node *NetworkNode, prefix string, isRoot bool, isLast bool) {
	if node == nil || (!node.Active && !isRoot) {
		return
	}

	if isRoot {
		fmt.Printf("%s%s\n", prefix, node.Name)
		childPrefix := prefix + "    "
		for i, child := range node.Children {
			printMeshNode(child, childPrefix, false, i == len(node.Children)-1)
		}
		return
	}

	connector := "├── "
	if isLast {
		connector = "└── "
	}

	icon := "💻"
	switch node.DeviceType {
	case "router":
		icon = "🌐 [FRITZ!Box]"
	case "repeater":
		icon = "📡 [Repeater]"
	}

	statusText := ""
	if !node.Active {
		statusText = " (inaktiv)"
	}

	if node.DeviceType == "client" {
		fmt.Printf("%s%s %s - %s (%s)%s\n", prefix, connector, node.Name, node.IP, node.MAC, statusText)
		return
	}

	fmt.Printf("%s%s %s %s - %s (%s)%s\n", prefix, connector, icon, node.Name, node.IP, node.MAC, statusText)

	childPrefix := prefix + "│   "
	if isLast {
		childPrefix = prefix + "    "
	}

	for i, child := range node.Children {
		printMeshNode(child, childPrefix, false, i == len(node.Children)-1)
	}
}
