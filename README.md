# Network Analyzer

Ein umfassendes Go-Programm zur Überwachung und Analyse von Netzwerktopologien, Geräteverbindungen und Netzwerkqualität. Ideal für die Überwachung von FRITZ!Box-basierten Heimnetzwerken mit Repeatern und mehreren Clients.

## 🎯 Features

### Analyzer (Hauptserver)
- **Automatische Subnet-Erkennung**: Erkennt das lokale Netzwerk automatisch
- **Echtzeit-Netzwerk-Scanning**: Kontinuierliche Überwachung aller Geräte
- **Protokoll-Tracking**: Automatische Erkennung von Diensten auf offenen Ports
- **Status-Monitoring**: Online/Offline-Status mit Fehlertoleranz
- **IP-Wechsel-Erkennung**: Verfolgung von Geräten, die die IP wechseln
- **WAN-Qualitätsmessung**: Monitoring von Latenz und Jitter zum Internet
- **Web-Dashboard**: Interaktives HTML5-Dashboard mit Live-Updates
- **Persistenz**: Automatisches Speichern von Netzwerkstatistiken
- **Mesh-Topologie**: SOAP-basierte Erkennung von FRITZ!Box und Repeatern
- **Logging**: Automatische Protokollierung von Netzwerkänderungen

### Agent
- **Dezentralisierte Überwachung**: Läuft auf einzelnen Geräten
- **Qualitätsmessung**: Latenz und Jitter zum Gateway
- **WiFi-Information**: RSSI-Wert und BSSID-Erkennung (Windows/Linux)
- **Auto-Discovery**: Automatische Erkennung des Analyzer-Servers
- **Self-Update**: Automatische Aktualisierungen vom Hauptserver
- **JSON-API**: Einfache REST-API für Abfragen

## 📋 Anforderungen

- **Go 1.16+**
- **Windows/Linux/macOS**
- **Netzwerkzugriff** zum lokalen Subnetz
- **FRITZ!Box** (für Mesh-Topologie-Features)
- **Administrator-Rechte** (für ARP-Tabellenzugriff)

## 🚀 Installation

### Projekt klonen
```bash
git clone <repository-url>
cd network-analyzer
```

### Abhängigkeiten
```bash
go mod download
```

### Kompilierung

**Analyzer:**
```bash
go build -o analyzer.exe .
```

**Agent:**
```bash
go build -o agent.exe ./agent
```

## 💻 Nutzung

### Analyzer starten

```bash
./analyzer.exe
```

**Ausgabe:**
```
Lokales Subnetz 192.168.178.0/24 erkannt.
Nutze Router 192.168.178.1 als DNS-Server.

═══════════════════════════════════════════════════════════
🚀 WEBDASHBOARD GESTARTET: http://localhost:8080
═══════════════════════════════════════════════════════════

>>> Überwachung gestartet.
>>> - Schnellprüfung:  alle 30 Sekunden
>>> - Komplett-Scan:   alle 5 Minuten
>>> - Dynamische Agenten-Erkennung & Updates: Aktiv
>>> (Abbruch mit Strg+C)
```

Besuche: **http://localhost:8080**

### Agent auf anderen Geräten starten

```bash
./agent.exe
```

**Ausgabe:**
```
🎯 Hauptserver dynamisch gefunden: http://192.168.178.10:8080
🚀 Agent v1.0 läuft auf 192.168.178.20:8081
```

## 🔧 Konfiguration

### Repeater-Namen anpassen

Bearbeite `analyzer/analyzer.go` und füge deine FRITZ!Box-MAC-Adressen ein:

```go
repeaterMap := map[string]string{
    "11:22:33:44:55:66": "🏠 FRITZ!Box (Basis)",
    "aa:bb:cc:dd:ee:ff": "🛋️ Wohnzimmer-Repeater",
    "99:88:77:66:55:44": "🏢 Keller / Büro-Repeater",
}
```

### Gescannte Ports anpassen

Bearbeite `analyzer/analyzer.go`:

```go
var commonPorts = []string{
    "80", "443", "8080", "8443", "5000", "631", "9000",
    "21", "22", "53", "135", "139", "445", "548",
    // Weitere Ports hinzufügen...
}
```

### WAN-Target ändern

Bearbeite `analyzer/analyzer.go` und `agent/agent.go`:

```go
const checkTarget = "8.8.8.8:53"  // Google DNS
```

## 📊 API-Endpunkte

### Analyzer

- **GET `/`** - Web-Dashboard
- **GET `/api/data`** - JSON mit allen Netzwerkdaten
- **GET `/api/is-main-server`** - Server-Identifikation
- **GET `/api/agent-version`** - Aktuelle Agent-Version
- **GET `/api/agent-binary`** - Agent-Binärdatei zum Download

### Agent

- **GET `/data`** - Agent-Daten (Latenz, WiFi-Info, etc.)

## 📁 Projektstruktur

```
network-analyzer/
├── main.go              # Analyzer Entry-Point
├── analyzer/
│   ├── main.go          # Agent Entry-Point
│   ├── agent.go         # Agent Package
│   └── agent.go         # Agent Package
├── analyzer/
│   ├── analyzer.go      # Analyzer Logik & Web-Server
│   └── meshtree.go      # FRITZ!Box Mesh-Topologie
├── static/
│   ├── index.html       # Web-Dashboard (Frontend)
│   ├── script.js        # JavaScript-Logik
│   └── styles.css       # Styling
├── go.mod              # Go-Abhängigkeiten
├── network_stats.json  # Persistierte Netzwerkdaten
└── network.log         # Änderungsprotokoll
```

## 🔄 Arbeitsablauf

1. **Analyzer** startet auf dem Hauptserver (Port 8080)
2. Analyzer erkennt das lokale Subnetz automatisch
3. Initialer Komplett-Scan alisiert alle Geräte
4. Alle 30 Sekunden: Schnellprüfung der bekannten IPs
5. Alle 5 Minuten: Komplett-Scan des gesamten Subnetzes
6. **Agent** auf Clients startet und findet Analyzer automatisch
7. Agent sendet Qualitätsmessungen (Latenz/Jitter/WiFi-Info)
8. Analyzer sammelt Agent-Daten und zeigt sie im Dashboard

## 📈 Datenerfassung

### Für jeden Host:
- IP-Adresse
- MAC-Adresse
- Hostname
- Aktive Services/Ports
- Latenz & Jitter
- Online/Offline-Status

### Historische Daten:
- Anzahl aktiver Geräte
- Durchschnittliche Latenz
- WAN-Latenz & Jitter
- Zeitstempel (500+ Einträge)

### Agent-Daten:
- Gerätename & IP
- WiFi-Signalstärke (RSSI)
- BSSID (Access Point)
- Latenz & Jitter
- Agent-Version

## 🐛 Troubleshooting

### Agent findet Server nicht
- Stelle sicher, dass Analyzer auf Port 8080 läuft
- Überprüfe, dass beide im gleichen Subnetz sind
- Firewall-Einstellungen prüfen

### ARP-Auflösung funktioniert nicht
- Windows: Admin-Rechte erforderlich
- Linux: `arp` Befehl installieren (`apt install net-tools`)

### FRITZ!Box wird nicht gefunden
- IP in `meshtree.go` prüfen (Standard: `192.168.178.1`)
- UPnP auf der Box aktivieren

### Agent aktualisiert sich nicht
- Überprüfe, ob die neue Agent-Binärdatei am Server vorhanden ist
- Schau in die Logs des Agents

## 📝 Logging

Alle Netzwerkänderungen werden in `network.log` protokolliert:

```
[2026-10-11 14:30:45] ✅ ONLINE : 192.168.178.42 (laptop) [aa:bb:cc:dd:ee:ff]
[2026-10-11 14:35:20] 🔄 IP-WECHSEL: Gerät tablet (aa:bb:cc:dd:ee:11) hat neue IP 192.168.178.50 (vorher 192.168.178.45)
[2026-10-11 14:42:10] ❌ OFFLINE: 192.168.178.30 (printer) [11:22:33:44:55:66] [nach 5 Fehlversuchen]
```

## 📦 Persistenz

Netzwerkstatistiken werden in `network_stats.json` gespeichert und beim Neustart wiederhergestellt:

```json
{
  "devices": [...],
  "history": [...],
  "agents": {...},
  "current_wan_latency": 25.5,
  "current_wan_jitter": 2.3
}
```

## 📄 Lizenz

Dieses Projekt ist unter der **Elastic License 2.0** lizenziert.

- ✅ Kostenlose kommerzielle Nutzung
- ✅ Modifizierung erlaubt
- ❌ Weitervertrieb/Sublizenzierung nicht erlaubt
- ❌ SaaS-Nutzung nicht erlaubt (ohne Genehmigung)

Siehe [LICENSE.md](LICENSE.md) für Details.

## 🤝 Beitragen

Bugfixes und Verbesserungen sind willkommen!

## 📧 Support

Für Fragen oder Probleme: Öffnen Sie ein Issue im Repository.

---

**Erstellt:** Oktober 2026  
**Version:** 1.0
