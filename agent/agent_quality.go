package main

import (
	"net"
	"time"
)

const checkTarget = "8.8.8.8:53"

func measureQuality() {
	var latencies []time.Duration
	for i := 0; i < 5; i++ {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", checkTarget, 500*time.Millisecond)
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
	rssi, bssid := getWifiInfo()

	dataMutex.Lock()
	currentData.Latency = avg
	currentData.Jitter = jitter
	currentData.WifiRSSI = rssi
	currentData.AccessPoint = bssid
	dataMutex.Unlock()
}
