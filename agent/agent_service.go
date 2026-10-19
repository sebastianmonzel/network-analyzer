package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

const listenPort = ":8081"

func start() {
	hostname, _ := os.Hostname()
	myIP := getLocalIP()

	dataMutex.Lock()
	currentData = AgentPayload{
		Hostname:    hostname,
		IP:          myIP,
		AccessPoint: "LAN / Unbekannt",
		Version:     AgentVersion,
	}
	dataMutex.Unlock()

	go func() {
		setServerBaseURL(discoverMainServer())
	}()

	go func() {
		measureQuality()
		for {
			time.Sleep(30 * time.Second)
			measureQuality()
		}
	}()

	go func() {
		time.Sleep(10 * time.Second)
		checkForUpdate()
		for {
			time.Sleep(10 * time.Minute)
			checkForUpdate()
		}
	}()

	http.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		dataMutex.RLock()
		defer dataMutex.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(currentData)
	})

	fmt.Printf("🚀 Agent v%s läuft auf %s%s\n", AgentVersion, myIP, listenPort)
	http.ListenAndServe(listenPort, nil)
}
