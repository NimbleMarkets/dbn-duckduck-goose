// Copyright (c) 2025 Neomantra Corp

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/NimbleMarkets/dbn-duckduck-goose/middleware"
	"github.com/NimbleMarkets/dbn-duckduck-goose/sdk"
	"github.com/gin-gonic/gin"
)

const defaultCountArg = 25

// Returns the last N trades by Dataset and Ticker.
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
//	@Router			/last-trades/json/{dataset}/{ticker} [get]
func GetLastTradesByDatasetAndTicker(c *gin.Context) {
	// extract parameters
	ticker, dataset, count, err := extractParamsTickerDatasetCount(c)
	if err != nil {
		middleware.BadRequestError(c, err)
		return
	}

	// perform the query
	results, err := queryLastTradesByDatasetAndTicker(ticker, dataset, count)
	if err != nil {
		errorMsg := fmt.Sprintf("query error for ticker:%s dataset:%s", ticker, dataset)
		middleware.InternalError(c, errorMsg, err)
		return
	}
	if len(results) == 0 {
		results = []*sdk.TradeTick{} // hack to return an empty array vs zero when no data is returned
	}
	c.JSON(http.StatusOK, results)
}

// Returns CSV file with last N trades by Dataset and Ticker
//
//	@Summary		GET last N trades by market and ticker
//	@ID				GetLastTradesByDatasetAndTickerCSV
//	@Description	Returns Excel file with last N trades by dataset and ticker.
//	@Produce		text/csv
//	@Param			dataset path string	true	"DataBento dataset" example(DBEQ.BASIC)
//	@Param			ticker path string	true	"symbol to query" example(AAPL)
//	@Param			count query integer	false	"(optional) number of trades to return - default is 25"
//	@Success		200	string      string "CSV file with last N trades"
//	@Failure		404	{object}	error "dataset not found"
//	@Failure		500	{object}	error
//	@Router			/last-trades/csv/{dataset}/{ticker} [get]
func GetLastTradesByDatasetAndTickerCSV(c *gin.Context) {
	// Extract parameters
	ticker, dataset, count, err := extractParamsTickerDatasetCount(c)
	if err != nil {
		middleware.BadRequestError(c, err)
		return
	}

	// Perform the query
	tempFile, err := copyLastTradesByDatasetAndTickerFormatted("csv", ticker, dataset, count)
	if err != nil {
		errorMsg := fmt.Sprintf("query error for ticker:%s dataset:%s", ticker, dataset)
		middleware.InternalError(c, errorMsg, err)
		return
	}
	c.Header("Content-Type", "text/csv")
	c.File(tempFile)

	// Delete the temporary file
	err = os.Remove(tempFile)
	if err != nil {
		c.Error(fmt.Errorf("failed to remove temp file: %s %w", tempFile, err))
		// this does not error-out the request though
	}
}

// Returns Excel file with last N trades by Dataset and Ticker
//
//	@Summary		GET last N trades by market and ticker
//	@ID				GetLastTradesByDatasetAndTickerExcel
//	@Description	Returns Excel file with last N trades by dataset and ticker.
//	@Produce		application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
//	@Param			dataset path string	true	"DataBento dataset" example(DBEQ.BASIC)
//	@Param			ticker path string	true	"symbol to query" example(AAPL)
//	@Param			count query integer	false	"(optional) number of trades to return - default is 25"
//	@Success		200	string      string "Excel file with last N trades"
//	@Failure		404	{object}	error "dataset not found"
//	@Failure		500	{object}	error
//	@Router			/last-trades/excel/{dataset}/{ticker} [get]
func GetLastTradesByDatasetAndTickerExcel(c *gin.Context) {
	// Extract parameters
	ticker, dataset, count, err := extractParamsTickerDatasetCount(c)
	if err != nil {
		middleware.BadRequestError(c, err)
		return
	}

	// Perform the query
	tempFile, err := copyLastTradesByDatasetAndTickerFormatted("xlsx", ticker, dataset, count)
	if err != nil {
		errorMsg := fmt.Sprintf("query error for ticker:%s dataset:%s", ticker, dataset)
		middleware.InternalError(c, errorMsg, err)
		return
	}

	// Transmit the file
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.File(tempFile)

	// Delete the temporary file
	err = os.Remove(tempFile)
	if err != nil {
		c.Error(fmt.Errorf("failed to remove temp file: %s %w", tempFile, err))
		// this does not error-out the request though
	}
}

// extractParamsTickerDatasetCount extracts the ticker, dataset, and count from the Param's request context.
// Includes a non-nil error, if any
func extractParamsTickerDatasetCount(c *gin.Context) (ticker string, dataset string, count int, err error) {
	ticker = c.Param("ticker")
	if ticker == "" {
		err = fmt.Errorf(":ticker cannot be empty")
		return "", "", 0, err
	}

	dataset = c.Param("dataset")
	if dataset == "" {
		err = fmt.Errorf(":dataset cannot be empty")
		return "", "", 0, err
	}

	count = defaultCountArg
	if countStr := c.Query("count"); countStr != "" {
		count, err = middleware.ValidatePositiveNonzeroInteger(countStr)
		if err != nil {
			err = fmt.Errorf("invalid 'count' in query string: %s. %w", countStr, err)
			return "", "", 0, err
		}
	}
	return ticker, dataset, count, nil
}

// queryLastTradesByDatasetAndTicker selects the trades from the database and returns it as an array of TradeTicks.
func queryLastTradesByDatasetAndTicker(ticker string, dataset string, count int) ([]*sdk.TradeTick, error) {
	if count <= 0 {
		count = defaultCountArg
	}
	queryStr := `SELECT timestamp, nanos, publisher, ticker, CAST(price AS DOUBLE) AS price, shares FROM trades
WHERE ticker = ? ORDER BY timestamp LIMIT ?;`

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

// copyLastTradesByDatasetAndTickerFormatted selects the trades from the database and copies it to the given format.
// Returns the temporary filename, or an error if any. It is the caller's responsibility to delete the file.
func copyLastTradesByDatasetAndTickerFormatted(format string, ticker string, dataset string, count int) (string, error) {
	// Grab a temporary file for destination
	tempFile, err := os.CreateTemp("", fmt.Sprintf("trades-*.%s", format))
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	// Perform the query
	if count <= 0 {
		count = defaultCountArg
	}
	queryStr := fmt.Sprintf(`COPY (SELECT MAKE_TIMESTAMP(CAST(timestamp AS BIGINT)*1_000_000) AS time, publisher, ticker, CAST(price AS DOUBLE) AS price, shares FROM trades
WHERE ticker = ? ORDER BY timestamp LIMIT ?)
TO '%s' WITH (FORMAT %s, HEADER true);`, tempFile.Name(), format)

	// execute the command on global DuckDB connection
	_, err = gDuckdbConn.ExecContext(context.Background(), queryStr, ticker, count)
	if err != nil {
		return "", fmt.Errorf("DuckDB query failed: %w", err)
	}

	return tempFile.Name(), nil
}
