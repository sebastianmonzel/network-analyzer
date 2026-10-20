package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

const listenPort = ":8081"

func start(ctx context.Context) error {
	return startWithReady(ctx, nil)
}

func startWithReady(ctx context.Context, ready chan<- error) (runErr error) {
	readySent := false
	defer func() {
		if ready != nil && !readySent {
			ready <- runErr
		}
	}()

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("hostname konnte nicht ermittelt werden: %w", err)
	}
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
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				measureQuality()
			}
		}
	}()

	go func() {
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		checkForUpdate()
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkForUpdate()
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		dataMutex.RLock()
		defer dataMutex.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(currentData); err != nil {
			fmt.Printf("Agent-Daten konnten nicht ausgegeben werden: %v\n", err)
		}
	})

	listener, err := net.Listen("tcp", listenPort)
	if err != nil {
		return fmt.Errorf("Agent konnte Port %s nicht öffnen: %w", listenPort, err)
	}
	if ready != nil {
		ready <- nil
		readySent = true
	}
	fmt.Printf("🚀 Agent v%s läuft auf %s%s\n", AgentVersion, myIP, listenPort)

	server := &http.Server{Handler: mux}
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("Agent-Webserver wurde beendet: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("Agent-Webserver konnte nicht sauber beendet werden: %w", err)
		}
		if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("Agent-Webserver wurde beim Beenden unerwartet beendet: %w", err)
		}
		return nil
	}
}
