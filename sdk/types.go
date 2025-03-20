// Copyright (c) 2025 Neomantra Corp
// types.go
//
// We put our SDK's types here.
//
// These are shared between the Go SDK and the OpenAPI generators

package sdk

// TradeTick is a trade tick event datum.
type TradeTick struct {
	Time   string  `json:"ts" example:"1713644400"`      // Trade event timestamp as seconds from the epoch
	Nanos  int64   `json:"ns" example:"123456"`          // Nanoseconds portion of the event timestamp
	Ticker string  `json:"sym,omitempty" example:"AAPL"` // Ticker of the trade
	Market string  `json:"mkt,omitempty" example:"Q"`    // Market of the trade
	Price  float64 `json:"px" example:"214.21"`          // Trade price
	Shares int64   `json:"sz" example:"100"`             // Trade size/volume
}
