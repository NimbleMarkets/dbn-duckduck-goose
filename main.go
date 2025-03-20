// Copyright (c) 2025 Neomantra Corp
//
// NOTE: this incurs billing, handle with care!
//

package main

import (
	"fmt"
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
)

///////////////////////////////////////////////////////////////////////////////

type Config struct {
	HostPort    string
	OutFilename string
	ApiKey      string
	Dataset     string
	STypeIn     dbn.SType
	Encoding    dbn.Encoding
	Schemas     []string
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

	config.STypeIn = dbn.SType_RawSymbol
	config.Encoding = dbn.Encoding_Dbn

	pflag.StringVarP(&config.HostPort, "hostport", "p", "localhost:8888", "'host:port' to service HTTP")
	pflag.StringVarP(&config.Dataset, "dataset", "d", "", "Dataset to subscribe to")
	pflag.StringArrayVarP(&config.Schemas, "schema", "s", []string{}, "Schema to subscribe to (multiple allowed)")
	pflag.StringVarP(&config.ApiKey, "key", "k", "", "Databento API key (or set 'DATABENTO_API_KEY' envvar)")
	pflag.StringVarP(&config.OutFilename, "out", "o", "", "Output filename for DBN stream ('-' for stdout)")
	pflag.VarP(&config.STypeIn, "sin", "i", "Input SType of the symbols. One of instrument_id, id, instr, raw_symbol, raw, smart, continuous, parent, nasdaq, cms")
	pflag.StringVarP(&startTimeArg, "start", "t", "", "Start time to request as ISO 8601 format (default: now)")
	pflag.BoolVarP(&config.Snapshot, "snapshot", "n", false, "Enable snapshot on subscription request")
	pflag.BoolVarP(&config.Verbose, "verbose", "v", false, "Verbose logging")
	pflag.BoolVarP(&showHelp, "help", "h", false, "Show help")
	pflag.Parse()

	// therese are pre-loaded subscription requests
	config.Symbols = pflag.Args()

	if showHelp {
		fmt.Fprintf(os.Stdout, "usage: %s -d <dataset> -s <schema> [opts] symbol1 symbol2 ...\n\n", os.Args[0])
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

	if len(config.Schemas) == 0 {
		fmt.Fprintf(os.Stderr, "requires at least --schema argument\n")
		os.Exit(1)
	}

	requireValOrExit(config.Dataset, "missing required --dataset")
	requireValOrExit(config.OutFilename, "missing required --out")

	// logger setup
	isRelease := (gin.Mode() == gin.ReleaseMode) // GIN_MODE="release"
	logger := middleware.CreateLogger("dbn-duckduck-goose", isRelease)
	defer logger.Sync()

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
	handlers.Register(config.HostPort, router, logger)

	// quick-and-dirty hoist the feed handler to a goroutine
	go func() {
		if err := runDbnLiveServer(config); err != nil {
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

func runDbnLiveServer(config Config) error {
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
		Encoding:             config.Encoding,
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
		for _, schema := range config.Schemas {
			subRequest := dbn_live.SubscriptionRequestMsg{
				Schema:   schema,
				StypeIn:  config.STypeIn,
				Symbols:  config.Symbols,
				Start:    config.StartTime,
				Snapshot: config.Snapshot,
			}
			if err = client.Subscribe(subRequest); err != nil {
				return fmt.Errorf("failed to subscribe LiveClient: %w", err)
			}
		}
	}

	// Start session
	if err = client.Start(); err != nil {
		return fmt.Errorf("failed to start LiveClient: %w", err)
	}

	if config.Encoding == dbn.Encoding_Dbn {
		return followStreamDBN(client, outWriter)
	} else {
		return followStreamJSON(client, outWriter)
	}
}

func followStreamDBN(client *dbn_live.LiveClient, outWriter io.Writer) error {
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

	// Follow the DBN stream, writing DBN messages to the file
	for dbnScanner.Next() {
		recordBytes := dbnScanner.GetLastRecord()[:dbnScanner.GetLastSize()]
		_, err := outWriter.Write(recordBytes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to write record: %s\n", err.Error())
			return err
		}
	}
	if err := dbnScanner.Error(); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "scanner err: %s\n", err.Error())
		return err
	}
	return nil
}

func followStreamJSON(client *dbn_live.LiveClient, outWriter io.Writer) error {
	// Get the JSON scanner
	jsonScanner := client.GetJsonScanner()
	if jsonScanner == nil {
		return fmt.Errorf("failed to get JsonScanner from LiveClient")
	}
	// Follow the JSON stream, writing JSON messages to the file
	for jsonScanner.Next() {
		recordBytes := jsonScanner.GetLastRecord()
		_, err := outWriter.Write(recordBytes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to write record: %s\n", err.Error())
			return err
		}
	}
	if err := jsonScanner.Error(); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "scanner err: %s\n", err.Error())
		return err
	}
	return nil
}
