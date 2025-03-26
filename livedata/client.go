// Copyright 2025 Neomantra Corp

package livedata

import (
	"database/sql"
	_ "embed" // Required for go:embed
	"fmt"
	"io"
	"os"

	"github.com/NimbleMarkets/dbn-duckduck-goose/middleware"

	"github.com/NimbleMarkets/dbn-go"
	dbn_live "github.com/NimbleMarkets/dbn-go/live"
)

var dbnLiveSchemas = []string{"trades", "ohlcv-1m"}

// LiveDataClient handles a DataBento live feed, writing records to a DuckDB
type LiveDataClient struct {
	config  LiveDataConfig
	started bool

	duckdbConn       *sql.DB
	tradesTableName  string
	candlesTableName string

	dbnClient    *dbn_live.LiveClient
	dbnVisitor   *LiveDataVisitor
	dbnSymbolMap *dbn.PitSymbolMap

	outWriter io.Writer
	outCloser func()
}

// NewLiveDataClient creates a new LiveDataClient for the given config and DuckDB connection.
// It will connect, authenticate, pre-subscribe any symbols, and start the streaming
// Returns nil and an error, if any
func NewLiveDataClient(config LiveDataConfig, duckdbConn *sql.DB) (*LiveDataClient, error) {
	// Create a new LiveDataClient, hooking up the visitor
	liveDataClient := &LiveDataClient{
		config:           config,
		started:          false,
		duckdbConn:       duckdbConn,
		tradesTableName:  "trades",
		candlesTableName: "candles",
	}
	liveDataClient.dbnVisitor = NewLiveDataVisitor(liveDataClient)

	// Create output file before connecting
	outWriter, outCloser, err := dbn.MakeCompressedWriter(config.OutFilename, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	liveDataClient.outWriter = outWriter
	liveDataClient.outCloser = outCloser
	closeOutCloser := true
	defer func() {
		// Clean up output file if something goes wrong before exit
		if closeOutCloser && liveDataClient.outCloser != nil {
			liveDataClient.outCloser()
			liveDataClient.outCloser = nil
		}
	}()

	// Create and connect LiveClient
	client, err := dbn_live.NewLiveClient(dbn_live.LiveConfig{
		ApiKey:               config.ApiKey,
		Dataset:              config.Dataset,
		Encoding:             dbn.Encoding_Dbn,
		SendTsOut:            false,
		VersionUpgradePolicy: dbn.VersionUpgradePolicy_AsIs,
		Verbose:              config.Verbose,
	})
	if err != nil {
		outCloser()
		return nil, fmt.Errorf("failed to create dbn_live.LiveClient: %w", err)
	}
	liveDataClient.dbnClient = client
	liveDataClient.dbnSymbolMap = dbn.NewPitSymbolMap()

	// Authenticate to server, this blocks
	if _, err = client.Authenticate(config.ApiKey); err != nil {
		return nil, fmt.Errorf("failed to authenticate dbn_live.LiveClient: %w", err)
	}

	// Pre-subscribe to symbols, this blocks
	if len(config.SubSymbols) != 0 {
		for _, schema := range dbnLiveSchemas {
			subRequest := dbn_live.SubscriptionRequestMsg{
				Schema:   schema,
				StypeIn:  dbn.SType_RawSymbol,
				Symbols:  config.SubSymbols,
				Start:    config.StartTime,
				Snapshot: config.Snapshot,
			}
			if err = client.Subscribe(subRequest); err != nil {
				return nil, fmt.Errorf("failed to subscribe LiveClient: %w", err)
			}
		}
	}

	// Run the templated database migrations on DuckDB
	err = middleware.RunMigration(duckdbConn, middleware.TradeMigrationTemplate, middleware.MigrationInfo{
		MigrationName: "tradeMigration",
		TableName:     liveDataClient.tradesTableName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run trade migration: %w", err)
	}
	err = middleware.RunMigration(duckdbConn, middleware.CandlesMigrationTemplate, middleware.MigrationInfo{
		MigrationName: "candleMigration",
		TableName:     liveDataClient.candlesTableName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run candle migration: %w", err)
	}

	// Start DataBento Live session
	if err = client.Start(); err != nil {
		return nil, fmt.Errorf("failed to start LiveClient: %w", err)
	}

	// Return the LiveDataClient
	closeOutCloser = false
	return liveDataClient, nil
}

// FollowStream listens to the DBN stream, handling records until it is stopped,
func (c *LiveDataClient) Stop() error {
	if !c.started {
		return nil
	}
	err := c.dbnClient.Stop()
	if err != nil {
		c.started = false
	}
	return nil
}

// runOutCloser runs the output file closer function, if it exists
func (c *LiveDataClient) runOutCloser() {
	if c.outCloser != nil {
		c.outCloser()
		c.outCloser = nil
	}
}

// FollowStream listens to the DBN stream, handling records until it is stopped.
// Only one of these should be invoked per client
func (c *LiveDataClient) FollowStream() error {
	if c.started {
		return fmt.Errorf("already started")
	}
	c.started = true
	defer c.runOutCloser() // Close the output file when done

	// Write metadata to file
	dbnScanner := c.dbnClient.GetDbnScanner()
	if dbnScanner == nil {
		return fmt.Errorf("failed to get DbnScanner from LiveClient")
	}
	metadata, err := dbnScanner.Metadata()
	if err != nil {
		return fmt.Errorf("failed to get metadata from LiveClient: %w", err)
	}
	if err = metadata.Write(c.outWriter); err != nil {
		return fmt.Errorf("failed to write metadata from LiveClient: %w", err)
	}

	// Initialize symbol map
	midTime := metadata.Start + (metadata.End-metadata.Start)/2
	c.dbnSymbolMap.FillFromMetadata(metadata, midTime)

	// Follow the DBN stream, writing DBN messages to the file
	for dbnScanner.Next() && c.started {
		// use the visitor to handle the record
		if err := dbnScanner.Visit(c.dbnVisitor); err != nil {
			return fmt.Errorf("failed to visit record: %w", err)
		}

		// Write the raw record to the log
		recordBytes := dbnScanner.GetLastRecord()[:dbnScanner.GetLastSize()]
		_, err := c.outWriter.Write(recordBytes)
		if err != nil {
			return fmt.Errorf("failed to write record: %w", err)
		}
	}

	if err := dbnScanner.Error(); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "scanner err: %s\n", err.Error())
		return err
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////

// LiveDataVisitor is the dbn.Visitor to dispatch the clients's message handlers
type LiveDataVisitor struct {
	c *LiveDataClient
}

// NewLiveDataVisitor creates is an implementation of all the dbn.Visitor interface.
func NewLiveDataVisitor(client *LiveDataClient) *LiveDataVisitor {
	return &LiveDataVisitor{c: client}
}

// OnMbp0 will insert the trade into the client's DuckDB
func (v *LiveDataVisitor) OnMbp0(tradeRecord *dbn.Mbp0Msg) error {
	timestamp, nanos := dbn.TimestampToSecNanos(tradeRecord.Header.TsEvent) // thanks dbn-go!
	micros := timestamp*1_000_000 + nanos/1_000
	ticker := v.c.dbnSymbolMap.Get(tradeRecord.Header.InstrumentID)

	sqlFormat := `INSERT INTO %s (date, timestamp, nanos, publisher, ticker, price, shares)
VALUES (MAKE_TIMESTAMP(%d)::DATE, %d, %d, %d, '%s', %f, %d)
ON CONFLICT DO NOTHING;`
	queryStr := fmt.Sprintf(sqlFormat, v.c.tradesTableName,
		micros, timestamp, nanos, tradeRecord.Header.PublisherID,
		ticker, dbn.Fixed9ToFloat64(tradeRecord.Price), tradeRecord.Size,
	)

	_, err := v.c.duckdbConn.Exec(queryStr)
	if err != nil {
		return fmt.Errorf("failed to insert trade: %w", err)
	}
	return nil
}

func (v *LiveDataVisitor) OnMbp10(record *dbn.Mbp10Msg) error {
	return nil
}

func (v *LiveDataVisitor) OnMbp1(record *dbn.Mbp1Msg) error {
	return nil
}

func (v *LiveDataVisitor) OnMbo(record *dbn.MboMsg) error {
	return nil
}

func (v *LiveDataVisitor) OnOhlcv(ohlcvRecord *dbn.OhlcvMsg) error {
	timestamp, nanos := dbn.TimestampToSecNanos(ohlcvRecord.Header.TsEvent) // thanks dbn-go!
	micros := timestamp*1_000_000 + nanos/1_000
	ticker := v.c.dbnSymbolMap.Get(ohlcvRecord.Header.InstrumentID)

	sqlFormat := `INSERT INTO %s (date, timestamp, nanos, publisher, ticker, volume, open, high, low, close)
VALUES (MAKE_TIMESTAMP(%d)::DATE, %d, %d, %d, '%s', %d, %f, %f, %f, %f)
ON CONFLICT DO NOTHING;`
	queryStr := fmt.Sprintf(sqlFormat, v.c.candlesTableName,
		micros, timestamp, nanos, ohlcvRecord.Header.PublisherID,
		ticker, ohlcvRecord.Volume,
		dbn.Fixed9ToFloat64(ohlcvRecord.Open),
		dbn.Fixed9ToFloat64(ohlcvRecord.High),
		dbn.Fixed9ToFloat64(ohlcvRecord.Low),
		dbn.Fixed9ToFloat64(ohlcvRecord.Close),
	)

	_, err := v.c.duckdbConn.Exec(queryStr)
	if err != nil {
		return fmt.Errorf("failed to execute insert candle: %w", err)
	}
	return nil
}

func (v *LiveDataVisitor) OnCbbo(record *dbn.CbboMsg) error {
	return nil
}

func (v *LiveDataVisitor) OnImbalance(record *dbn.ImbalanceMsg) error {
	return nil
}

func (v *LiveDataVisitor) OnStatMsg(record *dbn.StatMsg) error {
	return nil
}

func (v *LiveDataVisitor) OnStatusMsg(record *dbn.StatusMsg) error {
	return nil
}

func (v *LiveDataVisitor) OnInstrumentDefMsg(record *dbn.InstrumentDefMsg) error {
	return nil
}

func (v *LiveDataVisitor) OnErrorMsg(record *dbn.ErrorMsg) error {
	return nil
}

func (v *LiveDataVisitor) OnSystemMsg(record *dbn.SystemMsg) error {
	return nil
}

// OnSymbolMappingMsg will update the client's symbol map
func (v *LiveDataVisitor) OnSymbolMappingMsg(mappingRecord *dbn.SymbolMappingMsg) error {
	err := v.c.dbnSymbolMap.OnSymbolMappingMsg(mappingRecord)
	if err != nil {
		return fmt.Errorf("failed to handle SymbolMappingMsg: %w", err)
	}
	return nil
}

func (v *LiveDataVisitor) OnStreamEnd() error {
	return nil
}
