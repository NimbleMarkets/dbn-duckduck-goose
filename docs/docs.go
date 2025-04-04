// Package docs Code generated by swaggo/swag. DO NOT EDIT
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "termsOfService": "file:///dev/null",
        "contact": {
            "name": "Neomantra Corp",
            "url": "https://nimble.markets",
            "email": "nosupport@nimble.markets"
        },
        "license": {
            "name": "MIT",
            "url": "https://mit-license.org"
        },
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/candles/{dataset}/{ticker}": {
            "get": {
                "description": "Returns a time range of OHLCV for a Dataset and Ticker",
                "produces": [
                    "application/json"
                ],
                "summary": "Get a time range of OHLCV for a Dataset and Ticker",
                "operationId": "GetOhlcvByDatasetAndTicker",
                "parameters": [
                    {
                        "type": "string",
                        "example": "DBEQ.BASIC",
                        "description": "DataBento dataset",
                        "name": "dataset",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "example": "AAPL",
                        "description": "symbol to query",
                        "name": "ticker",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "format": "ISO8601",
                        "description": "(optional) start of date range in ISO8601. Default is midnight Eastern.",
                        "name": "start",
                        "in": "query"
                    },
                    {
                        "type": "string",
                        "format": "ISO8601",
                        "description": "(optional) end of date range in ISO8601. Default is now.",
                        "name": "end",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "array of Candles",
                        "schema": {
                            "type": "array",
                            "items": {
                                "$ref": "#/definitions/sdk.Candle"
                            }
                        }
                    },
                    "404": {
                        "description": "dataset not found",
                        "schema": {}
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {}
                    }
                }
            }
        },
        "/charts/candles/{dataset}/{ticker}": {
            "get": {
                "description": "Returns an HTML page candlestick chart with volume and EMA for the given dataset and ticker.",
                "produces": [
                    "text/html"
                ],
                "summary": "Returns an HTML page candlestick chart with volume and EMA for the given dataset and ticker.",
                "operationId": "GetCandleChartByDatasetAndTicker",
                "parameters": [
                    {
                        "type": "string",
                        "example": "DBEQ.BASIC",
                        "description": "DataBento dataset",
                        "name": "dataset",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "example": "AAPL",
                        "description": "symbol to query",
                        "name": "ticker",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "format": "ISO8601",
                        "description": "(optional) start of date range in ISO8601. Default is midnight Eastern.",
                        "name": "start",
                        "in": "query"
                    },
                    {
                        "type": "string",
                        "format": "ISO8601",
                        "description": "(optional) end of date range in ISO8601. Default is now.",
                        "name": "end",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "HTML page with candlestick chart",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "404": {
                        "description": "dataset not found",
                        "schema": {}
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {}
                    }
                }
            }
        },
        "/last-trades/csv/{dataset}/{ticker}": {
            "get": {
                "description": "Returns Excel file with last N trades by dataset and ticker.",
                "produces": [
                    "text/csv"
                ],
                "summary": "GET last N trades by market and ticker",
                "operationId": "GetLastTradesByDatasetAndTickerCSV",
                "parameters": [
                    {
                        "type": "string",
                        "example": "DBEQ.BASIC",
                        "description": "DataBento dataset",
                        "name": "dataset",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "example": "AAPL",
                        "description": "symbol to query",
                        "name": "ticker",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "integer",
                        "description": "(optional) number of trades to return - default is 25",
                        "name": "count",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "CSV file with last N trades",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "404": {
                        "description": "dataset not found",
                        "schema": {}
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {}
                    }
                }
            }
        },
        "/last-trades/excel/{dataset}/{ticker}": {
            "get": {
                "description": "Returns Excel file with last N trades by dataset and ticker.",
                "produces": [
                    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                ],
                "summary": "GET last N trades by market and ticker",
                "operationId": "GetLastTradesByDatasetAndTickerExcel",
                "parameters": [
                    {
                        "type": "string",
                        "example": "DBEQ.BASIC",
                        "description": "DataBento dataset",
                        "name": "dataset",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "example": "AAPL",
                        "description": "symbol to query",
                        "name": "ticker",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "integer",
                        "description": "(optional) number of trades to return - default is 25",
                        "name": "count",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "Excel file with last N trades",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "404": {
                        "description": "dataset not found",
                        "schema": {}
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {}
                    }
                }
            }
        },
        "/last-trades/json/{dataset}/{ticker}": {
            "get": {
                "description": "Returns the last N trades by dataset and ticker.",
                "produces": [
                    "application/json"
                ],
                "summary": "GET last N trades by market and ticker",
                "operationId": "GetLastTradesByDatasetAndTicker",
                "parameters": [
                    {
                        "type": "string",
                        "example": "DBEQ.BASIC",
                        "description": "DataBento dataset",
                        "name": "dataset",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "example": "AAPL",
                        "description": "symbol to query",
                        "name": "ticker",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "integer",
                        "description": "(optional) number of trades to return - default is 25",
                        "name": "count",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "array of TradeTicks",
                        "schema": {
                            "type": "array",
                            "items": {
                                "$ref": "#/definitions/sdk.TradeTick"
                            }
                        }
                    },
                    "404": {
                        "description": "dataset not found",
                        "schema": {}
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {}
                    }
                }
            }
        }
    },
    "definitions": {
        "sdk.Candle": {
            "type": "object",
            "properties": {
                "close": {
                    "description": "Close price of candlestick",
                    "type": "number",
                    "example": 214.21
                },
                "high": {
                    "description": "High price of candlestick",
                    "type": "number",
                    "example": 214.21
                },
                "low": {
                    "description": "Low price of candlestick",
                    "type": "number",
                    "example": 214.21
                },
                "ns": {
                    "description": "Nanoseconds portion of the event timestamp",
                    "type": "integer",
                    "example": 123456
                },
                "open": {
                    "description": "Open price of candlestick",
                    "type": "number",
                    "example": 214.21
                },
                "pub": {
                    "description": "DataBento Publisher ID",
                    "type": "integer",
                    "example": 1
                },
                "sym": {
                    "description": "Ticker of the trade",
                    "type": "string",
                    "example": "AAPL"
                },
                "ts": {
                    "description": "Trade event timestamp as seconds from the epoch",
                    "type": "integer",
                    "example": 1713644400
                },
                "volume": {
                    "description": "Volume in candlestick",
                    "type": "integer",
                    "example": 100
                }
            }
        },
        "sdk.TradeTick": {
            "type": "object",
            "properties": {
                "mkt": {
                    "description": "Market of the trade",
                    "type": "string",
                    "example": "Q"
                },
                "ns": {
                    "description": "Nanoseconds portion of the event timestamp",
                    "type": "integer",
                    "example": 123456
                },
                "pub": {
                    "description": "DataBento Publisher ID",
                    "type": "integer",
                    "example": 1
                },
                "px": {
                    "description": "Trade price",
                    "type": "number",
                    "example": 214.21
                },
                "sym": {
                    "description": "Ticker of the trade",
                    "type": "string",
                    "example": "AAPL"
                },
                "sz": {
                    "description": "Trade size/volume",
                    "type": "integer",
                    "example": 100
                },
                "ts": {
                    "description": "Trade event timestamp as seconds from the epoch",
                    "type": "integer",
                    "example": 1713644400
                }
            }
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "1.0",
	Host:             "api.example.com",
	BasePath:         "/api/v1",
	Schemes:          []string{"http"},
	Title:            "dbn-duckduck-goose",
	Description:      "DuckDB-backed DBN Golang web service",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
