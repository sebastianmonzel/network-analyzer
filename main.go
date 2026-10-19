package main

import (
	"fmt"
	"log"
	"time"

	"network-scanner/analyzer"
)

func main() {
	analyzer.LoadStatsFromFile()

	subnet, err := analyzer.GetLocalSubnet()
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

	networkState := make(map[string]analyzer.ScanResult)
	missCounts := make(map[string]int)

	go analyzer.StartWebServer()

	// Mesh-Topologie beim Start synchron aufbauen und anschließend ausgeben
	boxIP := routerIP
	analyzer.UpdateMeshTopology(boxIP)

	for i := 0; i < 20; i++ {
		stats := analyzer.GetStats()
		if stats != nil && stats.MeshTree != nil {
			fmt.Println()
			analyzer.PrintMeshTree(stats.MeshTree)
			fmt.Println()
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	if stats := analyzer.GetStats(); stats == nil || stats.MeshTree == nil {
		fmt.Println("[Mesh] Keine Topologie erkannt (FRITZ!Box nicht erreichbar oder keine Repeater/Clients gefunden).")
	}

	knownIPs := analyzer.RunScanCycle(allIPs, routerIP, networkState, missCounts, "INITIALER KOMPLETT-SCAN", true)

	fmt.Printf("\n>>> Überwachung gestartet.\n")
	fmt.Println(">>> - Schnellprüfung:  alle 30 Sekunden")
	fmt.Println(">>> - Komplett-Scan:   alle 5 Minuten")
	fmt.Println(">>> - Mesh-Update:     alle 10 Minuten")
	fmt.Println(">>> - Dynamische Agenten-Erkennung & Updates: Aktiv")
	fmt.Println(">>> (Abbruch mit Strg+C)")

	scanInterval := 30 * time.Second
	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()

	ticks := 0
	meshUpdateTicks := 0
	for range ticker.C {
		ticks++
		meshUpdateTicks++

		if meshUpdateTicks >= 20 {
			analyzer.UpdateMeshTopology(boxIP)
			meshUpdateTicks = 0
		}

		if ticks >= 10 {
			knownIPs = analyzer.RunScanCycle(allIPs, routerIP, networkState, missCounts, "5-MINUTEN KOMPLETT-SCAN", false)
			ticks = 0
		} else {
			knownIPs = analyzer.RunScanCycle(knownIPs, routerIP, networkState, missCounts, "SCHNELL-PRÜFUNG", false)
		}
	}
}
