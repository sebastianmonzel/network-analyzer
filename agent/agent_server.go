package main

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var cachedServerURL = "http://localhost:8080"
var serverMutex sync.RWMutex

func discoverMainServer() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "http://localhost:8080"
	}
	defer conn.Close()

	localIP := conn.LocalAddr().(*net.UDPAddr).IP.String()
	parts := strings.Split(localIP, ".")
	if len(parts) != 4 {
		return "http://localhost:8080"
	}

	basePrefix := fmt.Sprintf("%s.%s.%s.", parts[0], parts[1], parts[2])
	foundURL := make(chan string, 1)
	var wg sync.WaitGroup
	client := http.Client{Timeout: 400 * time.Millisecond}

	for i := 1; i < 255; i++ {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			url := fmt.Sprintf("http://%s:8080/api/is-main-server", ip)
			resp, err := client.Get(url)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					select {
					case foundURL <- fmt.Sprintf("http://%s:8080", ip):
					default:
					}
				}
			}
		}(fmt.Sprintf("%s%d", basePrefix, i))
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case url := <-foundURL:
		fmt.Printf("🎯 Hauptserver dynamisch gefunden: %s\n", url)
		return url
	case <-done:
		return "http://localhost:8080"
	case <-time.After(2 * time.Second):
		return "http://localhost:8080"
	}
}

func getServerBaseURL() string {
	serverMutex.RLock()
	url := cachedServerURL
	serverMutex.RUnlock()
	return url
}

func setServerBaseURL(url string) {
	serverMutex.Lock()
	cachedServerURL = url
	serverMutex.Unlock()
}
