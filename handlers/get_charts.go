// Copyright (c) 2025 Neomantra Corp

package handlers

import (
	"context"
	_ "embed" // Required for go:embed
	"fmt"
	"os"
	"time"

	"github.com/NimbleMarkets/dbn-duckduck-goose/middleware"
	"github.com/NimbleMarkets/dbn-duckduck-goose/sdk"
	"github.com/gin-gonic/gin"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
	"github.com/relvacode/iso8601"
)

// Embedded JavaScript file to customize Echarts

//go:embed js/xAxisFormatter.js
var xAxisFormatter string

//go:embed js/zoomLabelFormatter.js
var zoomLabelFormatter string

//go:embed js/tooltipFormatter.js
var tooltipFormatter string

//go:embed js/tooltipPositioner.js
var tooltipPositioner string

// TradeStat is where we store results from our VWAP trades query
type TradeStat struct {
	Timestamp   float64 `json:"ts" example:"1713644400"`       // Trade event timestamp as seconds from the epoch
	VwapPrice   float64 `json:"vwap" example:"214.21"`         // VWAP price
	VwapMA30min float64 `json:"vwapma_30min" example:"214.21"` // 30-minute moving average of VWAP
	VwapMA1hour float64 `json:"vwapma_1hour" example:"214.21"` // 1-hour moving average of VWAP
	VwapMA3hour float64 `json:"vwapma_3hour" example:"214.21"` // 3-hour moving average of VWAP
}

///////////////////////////////////////////////////////////////////////////////

// Returns an HTML page candlestick chart with volume and EMA for the given dataset and ticker.
//
//	@Summary		Returns an HTML page candlestick chart with volume and EMA for the given dataset and ticker.
//	@ID				GetCandleChartByDatasetAndTicker
//	@Description	Returns an HTML page candlestick chart with volume and EMA for the given dataset and ticker.
//	@Produce		html
//	@Param			dataset path string	true	"DataBento dataset" example(DBEQ.BASIC)
//	@Param			ticker path string	true	"symbol to query" example(AAPL)
//	@Param			start query string	false	"(optional) start of date range in ISO8601. Default is midnight Eastern." Format(ISO8601)
//	@Param			end query string	false	"(optional) end of date range in ISO8601. Default is now." Format(ISO8601)
//	@Success		200	{object}	string "HTML page with candlestick chart"
//	@Failure		404	{object}	error "dataset not found"
//	@Failure		500	{object}	error
//	@Router			/charts/candles/{dataset}/{ticker} [get]
func GetCandleChartByDatasetAndTicker(c *gin.Context) {
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
	queryStr := `SELECT timestamp, nanos, volume,
CAST(open AS DOUBLE), CAST(high AS DOUBLE), CAST(low AS DOUBLE), CAST(close AS DOUBLE)
FROM candles
WHERE ticker = ? AND timestamp BETWEEN ? AND ? ORDER BY timestamp;`
	rows, err := gDuckdbConn.QueryContext(context.Background(), queryStr, ticker, startTime.Unix(), endTime.Unix()+1)
	if err != nil {
		middleware.InternalError(c, fmt.Sprintf("candle query error for ticker:%s dataset:%s", ticker, dataset), err)
		return
	}
	defer rows.Close()

	var candles []*sdk.Candle
	for rows.Next() {
		candle := new(sdk.Candle)
		err := rows.Scan(&candle.Timestamp, &candle.Nanos, &candle.Volume,
			&candle.Open, &candle.High, &candle.Low, &candle.Close)
		if err != nil {
			middleware.InternalError(c, fmt.Sprintf("candle scan error for ticker:%s dataset:%s", ticker, dataset), err)
			return
		}
		candles = append(candles, candle)
	}

	// query for trades statistics
	queryStr = `
-- Calculate VWAP for each minute
WITH minute_vwap AS (
  SELECT 
    date_trunc('minute', MAKE_TIMESTAMP(CAST(timestamp AS BIGINT)*1_000_000)) AS minute_timestamp,
    SUM(price * shares) / SUM(shares) AS vwap,
    SUM(shares) AS volume
  FROM trades
  WHERE ticker = ? AND timestamp BETWEEN ? AND ?
  GROUP BY minute_timestamp
  ORDER BY minute_timestamp
)
-- Calculate moving averages on minute VWAP
SELECT 
  epoch(minute_timestamp) AS timestamp,
  vwap,
  -- 30-minute moving average of VWAP
  AVG(vwap) OVER (
    ORDER BY minute_timestamp 
    ROWS BETWEEN 29 PRECEDING AND CURRENT ROW
  ) AS ma_30min,
  -- 1-hour moving average of VWAP
  AVG(vwap) OVER (
    ORDER BY minute_timestamp 
    ROWS BETWEEN 59 PRECEDING AND CURRENT ROW
  ) AS ma_1hour,
  -- 3-hour moving average of VWAP
  AVG(vwap) OVER (
    ORDER BY minute_timestamp 
    ROWS BETWEEN 179 PRECEDING AND CURRENT ROW
  ) AS ma_3hour
FROM minute_vwap
ORDER BY minute_timestamp;`

	rows, err = gDuckdbConn.QueryContext(context.Background(), queryStr, ticker, startTime.Unix(), endTime.Unix()+1)
	if err != nil {
		middleware.InternalError(c, fmt.Sprintf("tradeStats query error for ticker:%s dataset:%s", ticker, dataset), err)
		return
	}
	defer rows.Close()

	var tradeStats []TradeStat
	for rows.Next() {
		tradeStat := TradeStat{}
		err := rows.Scan(&tradeStat.Timestamp, &tradeStat.VwapPrice,
			&tradeStat.VwapMA30min, &tradeStat.VwapMA1hour, &tradeStat.VwapMA3hour)
		if err != nil {
			middleware.InternalError(c, fmt.Sprintf("candle scan error for ticker:%s dataset:%s", ticker, dataset), err)
			return
		}
		tradeStats = append(tradeStats, tradeStat)
	}

	// Create candlestick
	chartFilename, err := createCandleChartHTML(ticker, dataset, candles, tradeStats)
	if err != nil {
		middleware.InternalError(c, fmt.Sprintf("candle generation failed for ticker:%s dataset:%s", ticker, dataset), err)
		return
	}

	// Transmit the file
	c.Header("Content-Type", "text/html")
	c.File(chartFilename)

	// Delete the temporary file
	err = os.Remove(chartFilename)
	if err != nil {
		c.Error(fmt.Errorf("failed to remove temp file: %s %w", chartFilename, err))
		// this does not error-out the request though
	}
}

///////////////////////////////////////////////////////////////////////////////

// createCandleChartHTML creates an ECharts chart HTML page with the given arguments.
// Returns the temporary filename, or an error if any. It is the caller's responsibility to delete the file.
func createCandleChartHTML(ticker string, dataset string, candles []*sdk.Candle, tradeStats []TradeStat) (string, error) {
	// chart title and subtitle
	chartTitle := fmt.Sprintf("%s Chart", ticker)
	chartSubtitle := dataset
	if len(candles) > 0 {
		chartSubtitle += fmt.Sprintf(" -- %s to %s",
			time.Unix(candles[0].Timestamp, 0).Format("2006-01-02 15:04:05"),
			time.Unix(candles[len(candles)-1].Timestamp, 0).Format("2006-01-02 15:04:05"))
	}

	// Create K-line (candlestick) chart and dataset
	klineChart := charts.NewKLine()
	klineChart.AddDataset(opts.Dataset{
		// NOTE: this marshals the full sdk.Candle value
		// One could make a local struct like TradeStats for a subset of the needed data instead
		Source: candles,
	}, opts.Dataset{
		Source: tradeStats,
	})
	klineChart.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			PageTitle: ticker,
			Theme:     "dark",
			Width:     "100%",
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    chartTitle,
			Subtitle: chartSubtitle,
		}),
		charts.WithGridOpts(
			opts.Grid{Left: "5%", Right: "5%", Height: "50%"},
			opts.Grid{Left: "5%", Right: "5%", Height: "15%", Top: "72%"},
		),
		charts.WithXAxisOpts(opts.XAxis{
			Type:      "category",
			GridIndex: 0,
			AxisLabel: &opts.AxisLabel{
				Show:      opts.Bool(true),
				Formatter: types.FuncStr(opts.FuncStripCommentsOpts(xAxisFormatter)),
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Type:      "value",
			Scale:     opts.Bool(true),
			GridIndex: 0,
			AxisLabel: &opts.AxisLabel{
				Show: opts.Bool(true),
			},
			AxisPointer: &opts.AxisPointer{
				Show: opts.Bool(true),
				Snap: opts.Bool(true),
			},
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:           "inside",
			XAxisIndex:     []int{0, 1},
			LabelFormatter: opts.FuncStripCommentsOpts(zoomLabelFormatter),
			Start:          0,
			End:            100,
		}, opts.DataZoom{
			Type:           "slider",
			XAxisIndex:     []int{0, 1},
			LabelFormatter: opts.FuncStripCommentsOpts(zoomLabelFormatter),
			Start:          0,
			End:            100,
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:        opts.Bool(true),
			Trigger:     "axis",
			AxisPointer: &opts.AxisPointer{Type: "line"},
			Formatter:   types.FuncStr(opts.FuncStripCommentsOpts(tooltipFormatter)),
			// TODO: https://github.com/go-echarts/go-echarts/pull/532
			// Position:    types.FuncStr(opts.FuncStripCommentsOpts(tooltipPositioner)),
		}),
	)

	// Setup the chart data
	klineChart.AddSeries("candles", nil,
		charts.WithItemStyleOpts(opts.ItemStyle{
			Color:        "#00da3c",
			Color0:       "#ec0000",
			BorderColor:  "#008F28",
			BorderColor0: "#8A0000"}),
		charts.WithEncodeOpts(opts.Encode{
			X: "ts",
			Y: [4]string{"open", "close", "low", "high"}, // that's the order ECharts needs
		}),
	)

	klineChart.ExtendXAxis(opts.XAxis{
		Type:        "category",
		SplitNumber: 20,

		GridIndex: 1,
		AxisTick:  &opts.AxisTick{Show: opts.Bool(false)},
		AxisLabel: &opts.AxisLabel{Show: opts.Bool(false)},
	})
	klineChart.ExtendYAxis(opts.YAxis{
		Type:        "value",
		Scale:       opts.Bool(true),
		GridIndex:   1,
		SplitNumber: 2,
		AxisLabel:   &opts.AxisLabel{Show: opts.Bool(true)},
		AxisLine:    &opts.AxisLine{Show: opts.Bool(true)},
		SplitLine:   &opts.SplitLine{Show: opts.Bool(true)},
	})

	volumeBarChart := charts.NewBar()
	volumeBarChart.AddSeries("volume", nil,
		charts.WithItemStyleOpts(opts.ItemStyle{Color: "#7fbe9e"}),
		charts.WithBarChartOpts(opts.BarChart{XAxisIndex: 1, YAxisIndex: 1}),
		charts.WithEncodeOpts(opts.Encode{X: "ts", Y: "volume"}))
	klineChart.Overlap(volumeBarChart)

	// https://coolors.co/0b58b1-fbff2a-ffad05-ee793e-d92906
	// color palette

	vwapLineChart := charts.NewLine()
	vwapLineChart.SetGlobalOptions(
		charts.WithXAxisOpts(opts.XAxis{SplitNumber: 20, GridIndex: 0}),
		charts.WithYAxisOpts(opts.YAxis{Scale: opts.Bool(true), GridIndex: 0}))
	vwapLineChart.AddSeries("vwap", nil,
		charts.WithLineStyleOpts(opts.LineStyle{Color: "#fbff2a", Opacity: 0.4, Type: "dashed"}),
		charts.WithItemStyleOpts(opts.ItemStyle{Color: "#fbff2a", Opacity: 0.4}),
		charts.WithLineChartOpts(opts.LineChart{XAxisIndex: 0, YAxisIndex: 0}),
		charts.WithDatasetIndex(1),
		charts.WithEncodeOpts(opts.Encode{X: "ts", Y: "vwap"}))
	klineChart.Overlap(vwapLineChart)

	vwapMA30minLineChart := charts.NewLine()
	vwapMA30minLineChart.SetGlobalOptions(
		charts.WithXAxisOpts(opts.XAxis{SplitNumber: 20, GridIndex: 0}),
		charts.WithYAxisOpts(opts.YAxis{Scale: opts.Bool(true), GridIndex: 0}))
	vwapMA30minLineChart.AddSeries("vwapMA-30min", nil,
		charts.WithLineStyleOpts(opts.LineStyle{Color: "#ffad05", Opacity: 0.4, Type: "dashed"}),
		charts.WithItemStyleOpts(opts.ItemStyle{Color: "#ffad05", Opacity: 0.4}),
		charts.WithLineChartOpts(opts.LineChart{XAxisIndex: 0, YAxisIndex: 0}),
		charts.WithDatasetIndex(1),
		charts.WithEncodeOpts(opts.Encode{X: "ts", Y: "vwapma_30min"}))
	klineChart.Overlap(vwapMA30minLineChart)

	vwapMA1hourLineChart := charts.NewLine()
	vwapMA1hourLineChart.SetGlobalOptions(
		charts.WithXAxisOpts(opts.XAxis{SplitNumber: 20, GridIndex: 0}),
		charts.WithYAxisOpts(opts.YAxis{Scale: opts.Bool(true), GridIndex: 0}))
	vwapMA1hourLineChart.AddSeries("vwapMA-1hour", nil,
		charts.WithLineStyleOpts(opts.LineStyle{Color: "#ee793e", Opacity: 0.4, Type: "dashed"}),
		charts.WithItemStyleOpts(opts.ItemStyle{Color: "#ee793e", Opacity: 0.4}),
		charts.WithLineChartOpts(opts.LineChart{XAxisIndex: 0, YAxisIndex: 0}),
		charts.WithDatasetIndex(1),
		charts.WithEncodeOpts(opts.Encode{X: "ts", Y: "vwapma_1hour"}))
	klineChart.Overlap(vwapMA1hourLineChart)

	vwapMA3hourLineChart := charts.NewLine()
	vwapMA3hourLineChart.SetGlobalOptions(
		charts.WithXAxisOpts(opts.XAxis{SplitNumber: 20, GridIndex: 0}),
		charts.WithYAxisOpts(opts.YAxis{Scale: opts.Bool(true), GridIndex: 0}))
	vwapMA3hourLineChart.AddSeries("vwapMA-3hour", nil,
		charts.WithLineStyleOpts(opts.LineStyle{Color: "#d92906", Opacity: 0.4, Type: "dashed"}),
		charts.WithItemStyleOpts(opts.ItemStyle{Color: "#d92906", Opacity: 0.4}),
		charts.WithLineChartOpts(opts.LineChart{XAxisIndex: 0, YAxisIndex: 0}),
		charts.WithDatasetIndex(1),
		charts.WithEncodeOpts(opts.Encode{X: "ts", Y: "vwapma_3hour"}))
	klineChart.Overlap(vwapMA3hourLineChart)

	// Where the magic happens!
	//
	// Grab a temporary file for destination
	tempFile, err := os.CreateTemp("", "charts-candle-*.html")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	// Render the chart HTML page to the file
	err = klineChart.Render(tempFile)
	if err != nil {
		return "", fmt.Errorf("failed to render chart: %w", err)
	}

	return tempFile.Name(), nil
}
