package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
)

const statsFileName = "network_stats.json"

func LoadStatsFromFile() {
	loadStatsFromFile()
}

func saveStatsToFile() {
	stats.RLock()
	data, err := json.MarshalIndent(stats, "", "  ")
	stats.RUnlock()

	if err == nil {
		os.WriteFile(statsFileName, data, 0644)
	}
}

func loadStatsFromFile() {
	file, err := os.ReadFile(statsFileName)
	if err != nil {
		return
	}

	stats.Lock()
	defer stats.Unlock()

	var loaded NetworkStats
	if err := json.Unmarshal(file, &loaded); err == nil {
		if loaded.History != nil {
			stats.History = loaded.History
		}
		if loaded.Devices != nil {
			stats.Devices = loaded.Devices
		}
		if loaded.Agents != nil {
			stats.Agents = loaded.Agents
		}
		fmt.Printf("📂 Vorherige Statistiken aus '%s' geladen (%d Historien-Einträge).\n", statsFileName, len(stats.History))
	}
}
