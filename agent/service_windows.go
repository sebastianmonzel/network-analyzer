//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const agentServiceName = "NetworkAnalyzerAgent"

func handleServiceCommand(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	switch strings.ToLower(args[0]) {
	case "service":
		return true, svc.Run(agentServiceName, &agentWindowsService{})
	case "install":
		return true, runServiceAction("Windows-Dienst installiert.", installAgentService)
	case "uninstall":
		return true, runServiceAction("Windows-Dienst entfernt.", uninstallAgentService)
	case "start":
		return true, runServiceAction("Windows-Dienst gestartet.", startAgentService)
	case "stop":
		return true, runServiceAction("Windows-Dienst gestoppt.", func() error {
			return controlAgentService(svc.Stop)
		})
	default:
		return true, fmt.Errorf("unbekannter Agent-Befehl %q; verfügbar: install, start, stop, uninstall", args[0])
	}
}

func runServiceAction(successMessage string, action func() error) error {
	if err := action(); err != nil {
		return err
	}
	fmt.Println(successMessage)
	return nil
}

type agentWindowsService struct{}

func (*agentWindowsService) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	ready := make(chan error, 1)
	go func() {
		done <- startWithReady(ctx, ready)
	}()

	if err := <-ready; err != nil {
		cancel()
		<-done
		return false, 1
	}
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case request := <-requests:
			if request.Cmd == svc.Stop || request.Cmd == svc.Shutdown {
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				if err := <-done; err != nil {
					return false, 1
				}
				return false, 0
			}
		case err := <-done:
			cancel()
			if err != nil {
				return false, 1
			}
			return false, 0
		}
	}
}

func installAgentService() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("Pfad der Agent-Datei konnte nicht ermittelt werden: %w", err)
	}
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("Verbindung zum Windows-Dienstemanager fehlgeschlagen (als Administrator ausführen): %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.CreateService(agentServiceName, executable, mgr.Config{
		DisplayName: "Network Analyzer Agent",
		Description: "Sammelt Netzwerkqualitätsdaten und stellt sie dem Network Analyzer bereit.",
		StartType:   mgr.StartAutomatic,
	}, "service")
	if err != nil {
		return fmt.Errorf("Windows-Dienst konnte nicht installiert werden: %w", err)
	}
	return service.Close()
}

func uninstallAgentService() error {
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("Verbindung zum Windows-Dienstemanager fehlgeschlagen (als Administrator ausführen): %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(agentServiceName)
	if err != nil {
		return fmt.Errorf("Windows-Dienst konnte nicht geöffnet werden: %w", err)
	}
	defer service.Close()

	status, err := service.Query()
	if err != nil {
		return fmt.Errorf("Status des Windows-Dienstes konnte nicht abgefragt werden: %w", err)
	}
	if status.State != svc.Stopped {
		if _, err := service.Control(svc.Stop); err != nil {
			return fmt.Errorf("Windows-Dienst konnte nicht gestoppt werden: %w", err)
		}
		if err := waitForServiceState(service, svc.Stopped); err != nil {
			return err
		}
	}
	if err := service.Delete(); err != nil {
		return fmt.Errorf("Windows-Dienst konnte nicht entfernt werden: %w", err)
	}
	return nil
}

func controlAgentService(command svc.Cmd) error {
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("Verbindung zum Windows-Dienstemanager fehlgeschlagen (als Administrator ausführen): %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(agentServiceName)
	if err != nil {
		return fmt.Errorf("Windows-Dienst ist nicht installiert: %w", err)
	}
	defer service.Close()

	status, err := service.Control(command)
	if err != nil {
		return fmt.Errorf("Windows-Dienst konnte nicht gesteuert werden: %w", err)
	}
	target := svc.Running
	if command == svc.Stop {
		target = svc.Stopped
	}
	if status.State != target {
		return waitForServiceState(service, target)
	}
	return nil
}

func startAgentService() error {
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("Verbindung zum Windows-Dienstemanager fehlgeschlagen (als Administrator ausführen): %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(agentServiceName)
	if err != nil {
		return fmt.Errorf("Windows-Dienst ist nicht installiert: %w", err)
	}
	defer service.Close()

	if err := service.Start(); err != nil {
		return fmt.Errorf("Windows-Dienst konnte nicht gestartet werden: %w", err)
	}
	return waitForServiceState(service, svc.Running)
}

func waitForServiceState(service *mgr.Service, target svc.State) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		status, err := service.Query()
		if err != nil {
			return fmt.Errorf("Status des Windows-Dienstes konnte nicht abgefragt werden: %w", err)
		}
		if status.State == target {
			return nil
		}
		if status.State == svc.Stopped && target != svc.Stopped {
			return fmt.Errorf("Windows-Dienst wurde unerwartet beendet (Win32-Exitcode %d, Dienst-Exitcode %d)", status.Win32ExitCode, status.ServiceSpecificExitCode)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("Zeitüberschreitung beim Warten auf den Dienststatus %v", target)
}
