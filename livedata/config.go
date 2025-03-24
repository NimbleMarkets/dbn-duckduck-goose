// Copyright 2025 Neomantra Corp

package livedata

import "time"

// LiveDataConfig is configuration data for our live service
type LiveDataConfig struct {
	OutFilename string    // Output filename for the DBN data (*.zst will be compressed)
	ApiKey      string    // DataBento API Key
	Dataset     string    // Databento Dataset to subscribe to
	SubSymbols  []string  // Symbols to automatically subscribe to
	StartTime   time.Time // Start time to request (default: now)
	Snapshot    bool      // Enable snapshot on subscription request
	Verbose     bool      // Verbose logging
}
