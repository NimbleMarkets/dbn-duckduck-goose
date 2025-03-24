// Copyright 2025 Neomantra Corp

package livedata

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"os"
	"text/template"

	"github.com/NimbleMarkets/dbn-go"
	dbn_live "github.com/NimbleMarkets/dbn-go/live"
)

var dbnLiveSchemas = []string{"trades"}

// LiveDataClient handles a DataBento live feed, writing records to a DuckDB
type LiveDataClient struct {
	config  LiveDataConfig
	started bool

	duckdbConn *sql.DB
	tableName  string

	dbnClient    *dbn_live.LiveClient
	dbnSymbolMap *dbn.PitSymbolMap

	outWriter io.Writer
	outCloser func()
}

// NewLiveDataClient creates a new LiveDataClient for the given config and DuckDB connection.
// It will connect, authenticate, pre-subscribe any symbols, and start the streaming
// Returns nil and an error, if any
func NewLiveDataClient(config LiveDataConfig, duckdbConn *sql.DB) (*LiveDataClient, error) {
	// Create a new LiveDataClient
	liveDataClient := &LiveDataClient{
		config:     config,
		started:    false,
		duckdbConn: duckdbConn,
		tableName:  "trades",
	}
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
	migrationTempl, err := template.New("tradeMigration").Parse(migrationTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to create template migration: %w", err)
	}
	var migrationBytes bytes.Buffer
	if err = migrationTempl.Execute(&migrationBytes, MigrationInfo{TableName: liveDataClient.tableName}); err != nil {
		return nil, fmt.Errorf("failed to template migration: %w", err)
	}
	_, err = duckdbConn.Exec(migrationBytes.String())
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
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

	// Setup symbol map
	midTime := metadata.Start + (metadata.End-metadata.Start)/2
	c.dbnSymbolMap.FillFromMetadata(metadata, midTime)

	// Follow the DBN stream, writing DBN messages to the file
	for dbnScanner.Next() && c.started {
		recordBytes := dbnScanner.GetLastRecord()[:dbnScanner.GetLastSize()]
		_, err := c.outWriter.Write(recordBytes)
		if err != nil {
			return fmt.Errorf("failed to write record: %w", err)
		}

		// write trade files to DuckDB
		header, err := dbnScanner.GetLastHeader()
		if err != nil {
			return fmt.Errorf("failed to read header: %w", err)
		}
		switch header.RType {
		case dbn.RType_Mbp0: // trade
			tradeRecord, err := dbn.DbnScannerDecode[dbn.Mbp0Msg](dbnScanner)
			if err != nil {
				return fmt.Errorf("failed to read Mbp0Msg: %w", err)
			}

			err = insertTrade(c.duckdbConn, c.tableName, tradeRecord, c.dbnSymbolMap)
			if err != nil {
				return fmt.Errorf("failed to insert trade: %w", err)
			}
		case dbn.RType_SymbolMapping: // symbol mapping
			mappingRecord, err := dbnScanner.DecodeSymbolMappingMsg()
			if err != nil {
				return fmt.Errorf("failed to read SymbolMappingMsg: %w", err)
			}
			err = c.dbnSymbolMap.OnSymbolMappingMsg(mappingRecord)
			if err != nil {
				return fmt.Errorf("failed to handle SymbolMappingMsg: %w", err)
			}
		}
	}
	if err := dbnScanner.Error(); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "scanner err: %s\n", err.Error())
		return err
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////

// MigrationInfo holds data to be injected by our migration template
type MigrationInfo struct {
	TableName string
}

// tradeMigrationTempl is the SQL format string
// Takes the "table_name"
var migrationTemplate string = `
-- Create trades table
CREATE TABLE IF NOT EXISTS {{.TableName}} (
	date date NOT NULL,
	timestamp integer NOT NULL,
	nanos integer NOT NULL,
	publisher integer NOT NULL,
	ticker varchar(12) NOT NULL,
	price decimal(19,3) NOT NULL,
	shares integer NOT NULL
);
-- Create indices
CREATE UNIQUE INDEX IF NOT EXISTS {{.TableName}}_publisher_date_ticker_timestamp_idx ON {{.TableName}} (publisher, date, ticker, timestamp);
CREATE UNIQUE INDEX IF NOT EXISTS {{.TableName}}_publisher_ticker_date_timestamp_idx ON {{.TableName}} (publisher, ticker, date, timestamp);
CREATE UNIQUE INDEX IF NOT EXISTS {{.TableName}}_publisher_timestamp_ticker_idx ON {{.TableName}} (publisher, timestamp, ticker);
CREATE UNIQUE INDEX IF NOT EXISTS {{.TableName}}_publisher_ticker_timestamp_idx ON {{.TableName}} (publisher, ticker, timestamp);
`

// insertTrade inserts a trade record into DuckDB
func insertTrade(duckdbConn *sql.DB, tableName string, tradeRecord *dbn.Mbp0Msg, dbnSymbolMap *dbn.PitSymbolMap) error {
	timestamp, nanos := dbn.TimestampToSecNanos(tradeRecord.Header.TsEvent) // thanks dbn-go!
	micros := timestamp + nanos/1_000
	ticker := dbnSymbolMap.Get(tradeRecord.Header.InstrumentID)

	sqlFormat := `INSERT INTO %s (date, timestamp, nanos, publisher, ticker, price, shares)
VALUES (MAKE_TIMESTAMP(%d), %d, %d, %d, '%s', %f, %d)
ON CONFLICT DO NOTHING;`
	queryStr := fmt.Sprintf(sqlFormat, tableName,
		micros, timestamp, nanos, tradeRecord.Header.PublisherID,
		ticker, dbn.Fixed9ToFloat64(tradeRecord.Price), tradeRecord.Size,
	)

	_, err := duckdbConn.Exec(queryStr)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}
	return nil
}
