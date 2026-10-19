package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
)

const AgentVersion = "1.0"

func checkForUpdate() {
	serverURL := getServerBaseURL()
	resp, err := http.Get(serverURL + "/api/agent-version")
	if err != nil {
		setServerBaseURL(discoverMainServer())
		return
	}
	defer resp.Body.Close()

	var result struct {
		Version string `json:"version"`
	}
	if json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	if result.Version != "" && result.Version != AgentVersion {
		fmt.Printf("🔄 Neue Agenten-Version %s entdeckt (aktuell: %s). Starte Update...\n", result.Version, AgentVersion)

		binResp, err := http.Get(serverURL + "/api/agent-binary")
		if err != nil {
			return
		}
		defer binResp.Body.Close()

		ex, err := os.Executable()
		if err != nil {
			return
		}

		tmpFile := ex + ".new"
		out, err := os.Create(tmpFile)
		if err != nil {
			return
		}
		io.Copy(out, binResp.Body)
		out.Close()

		os.Chmod(tmpFile, 0755)

		oldFile := ex + ".old"
		os.Remove(oldFile)

		if err := os.Rename(ex, oldFile); err != nil {
			return
		}

		if err := os.Rename(tmpFile, ex); err != nil {
			os.Rename(oldFile, ex)
			return
		}

		fmt.Println("✅ Update erfolgreich! Starte Agent neu...")
		cmd := exec.Command(ex, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Start()
		os.Exit(0)
	}
}
