// Copyright (c) 2025 Neomantra Corp

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/NimbleMarkets/dbn-duckduck-goose/middleware"
	"github.com/NimbleMarkets/dbn-duckduck-goose/sdk"
	"github.com/gin-gonic/gin"
	"github.com/relvacode/iso8601"
)

// Get a time range of OHLCV for a Dataset and Ticker
//
//	@Summary		Get a time range of OHLCV for a Dataset and Ticker
//	@ID				GetOhlcvByDatasetAndTicker
//	@Description	Returns a time range of OHLCV for a Dataset and Ticker
//	@Produce		json
//	@Param			dataset path string	true	"DataBento dataset" example(DBEQ.BASIC)
//	@Param			ticker path string	true	"symbol to query" example(AAPL)
//	@Param			start query string	false	"(optional) start of date range in ISO8601. Default is midnight Eastern." Format(ISO8601)
//	@Param			end query string	false	"(optional) end of date range in ISO8601. Default is now." Format(ISO8601)
//	@Success		200	{object}	[]sdk.Candle "array of Candles"
//	@Failure		404	{object}	error "dataset not found"
//	@Failure		500	{object}	error
//	@Router			/candles/{dataset}/{ticker} [get]
func GetOhlcvByDatasetAndTicker(c *gin.Context) {
	ticker := c.Param("ticker")
	if ticker == "" {
		middleware.BadRequestError(c, fmt.Errorf(":ticker cannot be empty"))
		return
	}

	dataset := c.Param("dataset")
	if dataset == "" {
		middleware.BadRequestError(c, fmt.Errorf(":dataset cannot be empty"))
		return
	}

	var startTime, endTime time.Time
	var err error
	if startStr := c.Query("start"); startStr == "" {
		year, month, day := middleware.NowEST().Date() // now in Eastern
		startTime = time.Date(year, month, day, 0, 0, 0, 0, middleware.EasternLocation())
	} else {
		startTime, err = iso8601.ParseString(startStr)
		if err != nil {
			middleware.BadRequestError(c, fmt.Errorf("invalid 'start' date format: %s. %w", startStr, err))
			return
		}
	}
	if endStr := c.Query("end"); endStr == "" {
		endTime = middleware.NowEST() // now in Eastern
	} else {
		endTime, err = iso8601.ParseString(endStr)
		if err != nil {
			middleware.BadRequestError(c, fmt.Errorf("invalid 'start' date format: %s. %w", endStr, err))
			return
		}
	}

	// query for candlesticks
	candles, err := queryCandlesByDatasetAndTicker(ticker, dataset, startTime, endTime)
	if err != nil {
		errorMsg := fmt.Sprintf("query error for ticker:%s dataset:%s", ticker, dataset)
		middleware.InternalError(c, errorMsg, err)
		return
	}
	if len(candles) == 0 {
		candles = []*sdk.Candle{} // hack to return an empty array vs zero when no data is returned
	}
	c.JSON(http.StatusOK, candles)

}

// queryCandlesByDatasetAndTicker selects the candles from the database
func queryCandlesByDatasetAndTicker(ticker string, dataset string, startTime time.Time, endTime time.Time) ([]*sdk.Candle, error) {
	queryStr := `SELECT timestamp, nanos, publisher, ticker, volume,
CAST(open AS DOUBLE), CAST(high AS DOUBLE), CAST(low AS DOUBLE), CAST(close AS DOUBLE)
FROM candles
WHERE ticker = ? AND timestamp BETWEEN ? AND ? ORDER BY timestamp DESC;`
	rows, err := gDuckdbConn.QueryContext(context.Background(), queryStr, ticker, startTime.Unix(), endTime.Unix()+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candles []*sdk.Candle
	for rows.Next() {
		candle := new(sdk.Candle)
		err := rows.Scan(&candle.Timestamp, &candle.Nanos, &candle.PublisherID, &candle.Ticker, &candle.Volume,
			&candle.Open, &candle.High, &candle.Low, &candle.Close)
		if err != nil {
			return nil, err
		}
		candles = append(candles, candle)
	}
	return candles, nil
}
