package analyzer

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

const currentAgentVersion = "1.0"
const agentBinaryFile = "agent_update" // Unter Windows: "agent_update.exe"

func StartWebServer() {
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFile(w, r, "static/index.html")
	})

	http.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats.RLock()
		defer stats.RUnlock()
		json.NewEncoder(w).Encode(stats)
	})

	http.HandleFunc("/api/mesh", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats.RLock()
		meshTree := stats.MeshTree
		stats.RUnlock()
		if meshTree == nil {
			w.Write([]byte(`{"mesh_tree": null}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"mesh_tree": meshTree})
	})

	http.HandleFunc("/api/is-main-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"server": "network-analyzer-master", "status": "active"}`))
	})

	http.HandleFunc("/api/agent-version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"version": currentAgentVersion})
	})

	http.HandleFunc("/api/agent-binary", func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(agentBinaryFile); os.IsNotExist(err) {
			http.Error(w, "Update-Datei nicht gefunden", http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, agentBinaryFile)
	})

	fmt.Println("\n=======================================================")
	fmt.Println("🚀 WEBDASHBOARD GESTARTET: http://localhost:8080")
	fmt.Println("=======================================================\n")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
