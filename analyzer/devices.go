package analyzer

import "strings"

var commonPorts = []string{
	"80", "443", "8080", "8443", "5000", "631", "9000",
	"21", "22", "53", "135", "139", "445", "548",
	"5353", "7000", "8009", "9100",
}

func getRepeaterName(bssid string) string {
	if bssid == "" || bssid == "-" || bssid == "LAN / Unbekannt" || bssid == "00:00:00:00:00:00" {
		return "🔌 LAN / Kabel"
	}

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
