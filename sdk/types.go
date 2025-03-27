// Copyright (c) 2025 Neomantra Corp
// types.go
//
// We put our SDK's types here.
//
// These are shared between the Go SDK and the OpenAPI generators

package sdk

// TradeTick is a trade tick event datum.
type TradeTick struct {
	Timestamp   int64   `json:"ts" example:"1713644400"`      // Trade event timestamp as seconds from the epoch
	Nanos       int64   `json:"ns" example:"123456"`          // Nanoseconds portion of the event timestamp
	PublisherID uint16  `json:"pub" example:"1"`              // DataBento Publisher ID
	Ticker      string  `json:"sym,omitempty" example:"AAPL"` // Ticker of the trade
	Market      string  `json:"mkt,omitempty" example:"Q"`    // Market of the trade
	Price       float64 `json:"px" example:"214.21"`          // Trade price
	Shares      int64   `json:"sz" example:"100"`             // Trade size/volume
}

// Candle is a OHLCV event datum.
type Candle struct {
	Timestamp   int64   `json:"ts" example:"1713644400"`      // Trade event timestamp as seconds from the epoch
	Nanos       int64   `json:"ns" example:"123456"`          // Nanoseconds portion of the event timestamp
	PublisherID uint16  `json:"pub" example:"1"`              // DataBento Publisher ID
	Ticker      string  `json:"sym,omitempty" example:"AAPL"` // Ticker of the trade
	Open        float64 `json:"open" example:"214.21"`        // Open price of candlestick
	High        float64 `json:"high" example:"214.21"`        // High price of candlestick
	Low         float64 `json:"low" example:"214.21"`         // Low price of candlestick
	Close       float64 `json:"close" example:"214.21"`       // Close price of candlestick
	Volume      uint64  `json:"volume" example:"100"`         // Volume in candlestick
}
