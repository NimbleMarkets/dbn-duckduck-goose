// Copyright (c) 2025 Neomantra Corp
//
// NOTE: this incurs billing, handle with care!
//

package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"time"

	dbn "github.com/NimbleMarkets/dbn-go"
	dbn_live "github.com/NimbleMarkets/dbn-go/live"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/penglongli/gin-metrics/ginmetrics"
	"github.com/relvacode/iso8601"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"github.com/NimbleMarkets/dbn-duckduck-goose/handlers"
	"github.com/NimbleMarkets/dbn-duckduck-goose/middleware"

	"database/sql"

	_ "github.com/marcboeker/go-duckdb"
)

///////////////////////////////////////////////////////////////////////////////

type Config struct {
	DuckDBFile  string
	HostPort    string
	OutFilename string
	ApiKey      string
	Dataset     string
	Symbols     []string
	StartTime   time.Time
	Snapshot    bool
	Verbose     bool
}

///////////////////////////////////////////////////////////////////////////////
// OpenAPI Documentation
//
//	@title			dbn-duckduck-goose
//	@version		1.0
//	@termsOfService	file:///dev/null
//	@description	DuckDB-backed DBN Golang web service
//
//	@contact.name	Neomantra Corp
//	@contact.url	https://nimble.markets
//	@contact.email	nosupport@nimble.markets
//
// @license.name	MIT
// @license.url		https://mit-license.org
//
//	@BasePath					/api/v1
//	@Host						api.example.com
//	@Schemes					http

func main() {
	var err error
	var config Config
	var startTimeArg string
	var showHelp bool

	pflag.StringVarP(&config.DuckDBFile, "db", "", "", "DuckDB datate file to use (default: ':memory:')")
	pflag.StringVarP(&config.HostPort, "hostport", "p", "localhost:8888", "'host:port' to service HTTP")
	pflag.StringVarP(&config.Dataset, "dataset", "d", "", "Dataset to subscribe to")
	pflag.StringVarP(&config.ApiKey, "key", "k", "", "Databento API key (or set 'DATABENTO_API_KEY' envvar)")
	pflag.StringVarP(&config.OutFilename, "out", "o", "", "Output filename for DBN stream ('-' for stdout)")
	pflag.StringVarP(&startTimeArg, "start", "t", "", "Start time to request as ISO 8601 format (default: now)")
	pflag.BoolVarP(&config.Snapshot, "snapshot", "n", false, "Enable snapshot on subscription request")
	pflag.BoolVarP(&config.Verbose, "verbose", "v", false, "Verbose logging")
	pflag.BoolVarP(&showHelp, "help", "h", false, "Show help")
	pflag.Parse()

	// therese are pre-loaded subscription requests
	config.Symbols = pflag.Args()

	if showHelp {
		fmt.Fprintf(os.Stdout, "usage: %s -d <dataset> [opts] symbol1 symbol2 ...\n\n", os.Args[0])
		pflag.PrintDefaults()
		os.Exit(0)
	}

	if startTimeArg != "" {
		config.StartTime, err = iso8601.ParseString(startTimeArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse --start as ISO 8601 time: %s\n", err.Error())
			os.Exit(1)
		}
	}

	if config.ApiKey == "" {
		config.ApiKey = os.Getenv("DATABENTO_API_KEY")
		requireValOrExit(config.ApiKey, "missing Databento API key, use --key or set DATABENTO_API_KEY envvar\n")
	}

	requireValOrExit(config.Dataset, "missing required --dataset")
	requireValOrExit(config.OutFilename, "missing required --out")

	// logger setup
	isRelease := (gin.Mode() == gin.ReleaseMode) // GIN_MODE="release"
	logger := middleware.CreateLogger("dbn-duckduck-goose", isRelease)
	defer logger.Sync()

	// DuckDB setup
	if config.DuckDBFile == "" {
		logger.Warn("no DuckDB file specified, using in-memory database")
	}
	duckdbConn, err := sql.Open("duckdb", config.DuckDBFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "duckdb failed to open: %s\n", err.Error())
		os.Exit(1)
	}
	defer duckdbConn.Close()

	// Gin webserver setup
	router := gin.New()
	router.Use(ginzap.Ginzap(logger, time.RFC3339, true))
	router.Use(ginzap.RecoveryWithZap(logger, true))
	router.Use(middleware.SetGinLogger(logger))
	router.Use(gin.Recovery()) // recover from panics and return 500

	// prometheus metrics middleware
	m := ginmetrics.GetMonitor()
	m.SetMetricPath("/metrics")
	// cutoff for slow request metric in seconds
	m.SetSlowTime(5)
	// request duration histogram buckets in seconds - used to p95, p99
	// default {0.1, 0.3, 1.2, 5, 10}
	m.SetDuration([]float64{0.1, 0.3, 1.2, 5, 10})
	m.Use(router)

	// Register our service's handlers/routes
	handlers.Register(config.HostPort, duckdbConn, router, logger)

	// quick-and-dirty hoist the feed handler to a goroutine
	go func() {
		if err := runDbnLiveServer(config, duckdbConn); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
			os.Exit(1)
		}
	}()

	// Run the web server in this goroutine
	logger.Info("Starting server", zap.String("hostport", config.HostPort))
	router.Run(config.HostPort)
	logger.Info("Server stopped")
}

// requireValOrExit exits with an error message if `val` is empty.
func requireValOrExit(val string, errstr string) {
	if val == "" {
		fmt.Fprintf(os.Stderr, "%s\n", errstr)
		os.Exit(1)
	}
}

///////////////////////////////////////////////////////////////////////////////

var dbnLiveSchemas = []string{"trades"}

func runDbnLiveServer(config Config, duckdbConn *sql.DB) error {
	// Create output file before connecting
	outWriter, outCloser, err := dbn.MakeCompressedWriter(config.OutFilename, false)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outCloser()

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
		return fmt.Errorf("failed to create LiveClient: %w", err)
	}
	defer client.Stop()

	// Authenticate to server
	if _, err = client.Authenticate(config.ApiKey); err != nil {
		return fmt.Errorf("failed to authenticate LiveClient: %w", err)
	}

	// Pre-subscribe to symbols
	if len(config.Symbols) != 0 {
		for _, schema := range dbnLiveSchemas {
			subRequest := dbn_live.SubscriptionRequestMsg{
				Schema:   schema,
				StypeIn:  dbn.SType_RawSymbol,
				Symbols:  config.Symbols,
				Start:    config.StartTime,
				Snapshot: config.Snapshot,
			}
			if err = client.Subscribe(subRequest); err != nil {
				return fmt.Errorf("failed to subscribe LiveClient: %w", err)
			}
		}
	}

	// Run the templated database migrations on DuckDB
	tableName := "trades"
	migrationTempl, err := template.New("tradeMigration").Parse(migrationTemplate)
	if err != nil {
		return fmt.Errorf("failed to create template migration: %w", err)
	}
	var migrationBytes bytes.Buffer
	if err = migrationTempl.Execute(&migrationBytes, MigrationInfo{TableName: tableName}); err != nil {
		return fmt.Errorf("failed to template migration: %w", err)
	}
	_, err = duckdbConn.Exec(migrationBytes.String())
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	// Start DataBento Live session
	if err = client.Start(); err != nil {
		return fmt.Errorf("failed to start LiveClient: %w", err)
	}

	return followStreamDBN(client, outWriter, duckdbConn, "trades")
}

func followStreamDBN(client *dbn_live.LiveClient, outWriter io.Writer, duckdbConn *sql.DB, tableName string) error {
	// Write metadata to file
	dbnScanner := client.GetDbnScanner()
	if dbnScanner == nil {
		return fmt.Errorf("failed to get DbnScanner from LiveClient")
	}
	metadata, err := dbnScanner.Metadata()
	if err != nil {
		return fmt.Errorf("failed to get metadata from LiveClient: %w", err)
	}
	if err = metadata.Write(outWriter); err != nil {
		return fmt.Errorf("failed to write metadata from LiveClient: %w", err)
	}

	// Setup symbol map
	dbnSymbolMap := dbn.NewPitSymbolMap()
	midTime := metadata.Start + (metadata.End-metadata.Start)/2
	dbnSymbolMap.FillFromMetadata(metadata, midTime)

	// Follow the DBN stream, writing DBN messages to the file
	for dbnScanner.Next() {
		recordBytes := dbnScanner.GetLastRecord()[:dbnScanner.GetLastSize()]
		_, err := outWriter.Write(recordBytes)
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

			err = insertTrade(duckdbConn, tableName, tradeRecord, dbnSymbolMap)
			if err != nil {
				return fmt.Errorf("failed to insert trade: %w", err)
			}
		case dbn.RType_SymbolMapping: // symbol mapping
			mappingRecord, err := dbnScanner.DecodeSymbolMappingMsg()
			if err != nil {
				return fmt.Errorf("failed to read SymbolMappingMsg: %w", err)
			}
			err = dbnSymbolMap.OnSymbolMappingMsg(mappingRecord)
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
