// Copyright (c) 2025 Neomantra Corp

package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/NimbleMarkets/dbn-duckduck-goose/middleware"
	"github.com/NimbleMarkets/dbn-duckduck-goose/sdk"
	"github.com/gin-gonic/gin"
)

const defaultCountArg = 25

// Get last trades by Dataset and Ticker
//
//	@Summary		GET last N trades by market and ticker
//	@ID				GetLastTradesByDatasetAndTicker
//	@Description	Returns the last N trades by dataset and ticker.
//	@Produce		json
//	@Param			dataset path string	true	"DataBento dataset" example(DBEQ.BASIC)
//	@Param			ticker path string	true	"symbol to query" example(AAPL)
//	@Param			count query integer	false	"(optional) number of trades to return - default is 25"
//	@Success		200	{object}	[]sdk.TradeTick "array of TradeTicks"
//	@Failure		404	{object}	error "dataset not found"
//	@Failure		500	{object}	error
//	@Router			/last-trades/{dataset}/{ticker} [get]
func GetLastTradesByDatasetAndTicker(c *gin.Context) {
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

	count := defaultCountArg
	if countStr := c.Query("count"); countStr != "" {
		validatedCount, err := middleware.ValidatePositiveNonzeroInteger(countStr)
		if err != nil {
			middleware.BadRequestError(c, fmt.Errorf("invalid 'count' in query string: %s. %w", countStr, err))
			return
		}
		count = validatedCount
	}

	// perform the query
	results, err := queryLastTradesByDatasetAndTicker(ticker, dataset, count)
	if err != nil {
		errorMsg := fmt.Sprintf("getTradeTicksByTickerMarket error for ticker:%s dataset:%s", ticker, dataset)
		middleware.InternalError(c, errorMsg, err)
		return
	}
	if len(results) == 0 {
		results = []*sdk.TradeTick{} // hack to return an empty array vs zero when no data is returned
	}
	c.JSON(http.StatusOK, results)
}

// queryLastTradesByDatasetAndTicker selects the trades from the database
func queryLastTradesByDatasetAndTicker(ticker string, dataset string, count int) ([]*sdk.TradeTick, error) {
	if count <= 0 {
		count = defaultCountArg
	}
	queryStr := `SELECT timestamp, nanos, publisher, ticker, CAST(price AS DOUBLE), shares FROM trades
WHERE ticker = ? ORDER BY timestamp DESC LIMIT ?;`

	// query the global DuckDB connection
	rows, err := gDuckdbConn.QueryContext(context.Background(), queryStr, ticker, count)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ticks []*sdk.TradeTick
	for rows.Next() {
		tick := new(sdk.TradeTick)
		err := rows.Scan(&tick.Timestamp, &tick.Nanos, &tick.PublisherID, &tick.Ticker, &tick.Price, &tick.Shares)
		if err != nil {
			return nil, err
		}
		ticks = append(ticks, tick)
	}
	return ticks, nil
}
