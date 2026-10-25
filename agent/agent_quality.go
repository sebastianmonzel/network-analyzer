package main

import (
	"fmt"
	"net"
	"time"
)

const checkTarget = "8.8.8.8:53"

func measureQuality() {
	var latencies []time.Duration
	const attempts = 5
	for i := 0; i < attempts; i++ {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", checkTarget, 500*time.Millisecond)
		if err == nil {
			latencies = append(latencies, time.Since(start))
			if err := conn.Close(); err != nil {
				fmt.Printf("Messverbindung konnte nicht geschlossen werden: %v\n", err)
			}
		}
		if i < attempts-1 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	checkedAt := time.Now().UTC()
	rssi, bssid := getWifiInfo()
	recordQualityMeasurement(latencies, attempts, checkedAt, rssi, bssid)
}

func recordQualityMeasurement(latencies []time.Duration, attempts int, checkedAt time.Time, rssi int, bssid string) {
	dataMutex.Lock()
	defer dataMutex.Unlock()
	currentData.LastCheckedAt = checkedAt.Format(time.RFC3339)
	currentData.ProbeAttempts = attempts
	currentData.ProbeSuccesses = len(latencies)
	currentData.WifiRSSI = rssi
	currentData.AccessPoint = bssid
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

	currentData.Latency = float64(total) / float64(len(latencies)) / float64(time.Millisecond)
	currentData.Jitter = float64(maxL-minL) / float64(time.Millisecond)
	currentData.LastMeasurementAt = checkedAt.Format(time.RFC3339)
}
