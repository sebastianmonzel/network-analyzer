package main

import (
	"testing"
	"time"
)

func TestRecordQualityMeasurementTracksFailuresAndKeepsLastSuccess(t *testing.T) {
	original := currentData
	t.Cleanup(func() {
		currentData = original
	})
	currentData = AgentPayload{}

	firstCheck := time.Date(2026, time.October, 22, 20, 0, 0, 0, time.UTC)
	recordQualityMeasurement(
		[]time.Duration{20 * time.Millisecond, 30 * time.Millisecond},
		5,
		firstCheck,
		-60,
		"AP-1",
	)
	if currentData.ProbeAttempts != 5 || currentData.ProbeSuccesses != 2 {
		t.Fatalf("probe result = %d/%d, want 2/5", currentData.ProbeSuccesses, currentData.ProbeAttempts)
	}
	if currentData.LastMeasurementAt != firstCheck.Format(time.RFC3339) {
		t.Fatalf("last measurement = %q, want %q", currentData.LastMeasurementAt, firstCheck.Format(time.RFC3339))
	}
	if currentData.Latency != 25 || currentData.Jitter != 10 {
		t.Fatalf("latency/jitter = %.1f/%.1f, want 25/10", currentData.Latency, currentData.Jitter)
	}

	failedCheck := firstCheck.Add(time.Minute)
	recordQualityMeasurement(nil, 5, failedCheck, -72, "AP-2")
	if currentData.ProbeAttempts != 5 || currentData.ProbeSuccesses != 0 {
		t.Fatalf("failed probe result = %d/%d, want 0/5", currentData.ProbeSuccesses, currentData.ProbeAttempts)
	}
	if currentData.LastCheckedAt != failedCheck.Format(time.RFC3339) {
		t.Fatalf("last check = %q, want %q", currentData.LastCheckedAt, failedCheck.Format(time.RFC3339))
	}
	if currentData.LastMeasurementAt != firstCheck.Format(time.RFC3339) {
		t.Fatalf("failed check overwrote last successful measurement timestamp: %q", currentData.LastMeasurementAt)
	}
	if currentData.Latency != 25 || currentData.Jitter != 10 {
		t.Fatalf("failed check overwrote last successful metrics: %.1f/%.1f", currentData.Latency, currentData.Jitter)
	}
	if currentData.WifiRSSI != -72 || currentData.AccessPoint != "AP-2" {
		t.Fatalf("latest Wi-Fi data = %d/%q, want -72/AP-2", currentData.WifiRSSI, currentData.AccessPoint)
	}
}
