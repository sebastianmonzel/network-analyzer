package analyzer

import (
	"testing"
	"time"
)

func TestRecordWANMeasurementTracksFailuresAndKeepsLastSuccess(t *testing.T) {
	stats.Lock()
	originalLatency := stats.CurrentWanLatency
	originalJitter := stats.CurrentWanJitter
	originalCheckedAt := stats.WanLastCheckedAt
	originalMeasuredAt := stats.WanLastMeasurementAt
	originalAttempts := stats.WanProbeAttempts
	originalSuccesses := stats.WanProbeSuccesses
	stats.Unlock()
	t.Cleanup(func() {
		stats.Lock()
		stats.CurrentWanLatency = originalLatency
		stats.CurrentWanJitter = originalJitter
		stats.WanLastCheckedAt = originalCheckedAt
		stats.WanLastMeasurementAt = originalMeasuredAt
		stats.WanProbeAttempts = originalAttempts
		stats.WanProbeSuccesses = originalSuccesses
		stats.Unlock()
	})

	stats.Lock()
	stats.CurrentWanLatency = 0
	stats.CurrentWanJitter = 0
	stats.WanLastCheckedAt = ""
	stats.WanLastMeasurementAt = ""
	stats.WanProbeAttempts = 0
	stats.WanProbeSuccesses = 0
	stats.Unlock()

	firstCheck := time.Date(2026, time.October, 22, 20, 0, 0, 0, time.UTC)
	recordWANMeasurement(
		[]time.Duration{20 * time.Millisecond, 30 * time.Millisecond},
		5,
		firstCheck,
	)
	stats.RLock()
	if stats.WanProbeAttempts != 5 || stats.WanProbeSuccesses != 2 {
		t.Fatalf("probe result = %d/%d, want 2/5", stats.WanProbeSuccesses, stats.WanProbeAttempts)
	}
	if stats.CurrentWanLatency != 25 || stats.CurrentWanJitter != 10 {
		t.Fatalf("latency/jitter = %.1f/%.1f, want 25/10", stats.CurrentWanLatency, stats.CurrentWanJitter)
	}
	stats.RUnlock()

	failedCheck := firstCheck.Add(time.Minute)
	recordWANMeasurement(nil, 5, failedCheck)
	stats.RLock()
	defer stats.RUnlock()
	if stats.WanProbeAttempts != 5 || stats.WanProbeSuccesses != 0 {
		t.Fatalf("failed probe result = %d/%d, want 0/5", stats.WanProbeSuccesses, stats.WanProbeAttempts)
	}
	if stats.WanLastCheckedAt != failedCheck.Format(time.RFC3339) {
		t.Fatalf("last check = %q, want %q", stats.WanLastCheckedAt, failedCheck.Format(time.RFC3339))
	}
	if stats.WanLastMeasurementAt != firstCheck.Format(time.RFC3339) {
		t.Fatalf("failed check overwrote last successful timestamp: %q", stats.WanLastMeasurementAt)
	}
	if stats.CurrentWanLatency != 25 || stats.CurrentWanJitter != 10 {
		t.Fatalf("failed check overwrote last successful metrics: %.1f/%.1f", stats.CurrentWanLatency, stats.CurrentWanJitter)
	}
}
