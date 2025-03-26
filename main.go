// Copyright (c) 2025 Neomantra Corp
//
// NOTE: this incurs billing, handle with care!
//

package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/penglongli/gin-metrics/ginmetrics"
	"github.com/relvacode/iso8601"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"github.com/NimbleMarkets/dbn-duckduck-goose/handlers"
	"github.com/NimbleMarkets/dbn-duckduck-goose/livedata"
	"github.com/NimbleMarkets/dbn-duckduck-goose/middleware"

	"database/sql"

	_ "github.com/marcboeker/go-duckdb/v2"
)

///////////////////////////////////////////////////////////////////////////////

type ServiceConfig struct {
	HostPort   string                  // HostPort to server the webserver on
	DuckDBFile string                  // DuckDB file to connect to (default: ':memory:')
	LiveConfig livedata.LiveDataConfig // LiveDataConfig configuration
	Verbose    bool                    // Verbose logging
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
	var config ServiceConfig
	var startTimeArg string
	var showHelp bool

	pflag.StringVarP(&config.DuckDBFile, "db", "", "", "DuckDB datate file to use (default: ':memory:')")
	pflag.StringVarP(&config.HostPort, "hostport", "p", "localhost:8888", "'host:port' to service HTTP")
	pflag.StringVarP(&config.LiveConfig.Dataset, "dataset", "d", "", "Dataset to subscribe to")
	pflag.StringVarP(&config.LiveConfig.ApiKey, "key", "k", "", "Databento API key (or set 'DATABENTO_API_KEY' envvar)")
	pflag.StringVarP(&config.LiveConfig.OutFilename, "out", "o", "", "Output filename for DBN stream ('-' for stdout)")
	pflag.StringVarP(&startTimeArg, "start", "t", "", "Start time to request as ISO 8601 format (default: now)")
	pflag.BoolVarP(&config.LiveConfig.Snapshot, "snapshot", "n", false, "Enable snapshot on subscription request")
	pflag.BoolVarP(&config.Verbose, "verbose", "v", false, "Verbose logging")
	pflag.BoolVarP(&showHelp, "help", "h", false, "Show help")
	pflag.Parse()

	// therese are pre-loaded subscription requests
	config.LiveConfig.SubSymbols = pflag.Args()
	config.LiveConfig.Verbose = config.Verbose

	if showHelp {
		fmt.Fprintf(os.Stdout, "usage: %s -d <dataset> [opts] symbol1 symbol2 ...\n\n", os.Args[0])
		pflag.PrintDefaults()
		os.Exit(0)
	}

	if startTimeArg != "" {
		config.LiveConfig.StartTime, err = iso8601.ParseString(startTimeArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse --start as ISO 8601 time: %s\n", err.Error())
			os.Exit(1)
		}
	}

	if config.LiveConfig.ApiKey == "" {
		config.LiveConfig.ApiKey = os.Getenv("DATABENTO_API_KEY")
		requireValOrExit(config.LiveConfig.ApiKey, "missing Databento API key, use --key or set DATABENTO_API_KEY envvar\n")
	}

	requireValOrExit(config.LiveConfig.Dataset, "missing required --dataset")
	requireValOrExit(config.LiveConfig.OutFilename, "missing required --out")

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

	// Create our LiveDataClient
	liveDataClient, err := livedata.NewLiveDataClient(config.LiveConfig, duckdbConn)
	if err != nil {
		logger.Error("failed to create LiveDataClient", zap.Error(err))
		os.Exit(1)
	}

	// Run the LiveDataClient in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("LiveDataClient following DataBento Live Stream", zap.String("dataset", config.LiveConfig.Dataset))
		if err := liveDataClient.FollowStream(); err != nil {
			logger.Error("LiveDataClient error:", zap.Error(err))
		}
	}()

	// Run the web server in a goroutine
	go func() {
		// we can add graceful shutdown with this:
		// https://gin-gonic.com/docs/examples/graceful-restart-or-stop/
		logger.Info("Running web server", zap.String("hostport", config.HostPort))
		router.Run(config.HostPort)
		logger.Info("Web server stopped")
	}()

	// Wait for interrupt signal to shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Signal received, shutting down...")

	liveDataClient.Stop()
	wg.Wait() // wait for LiveDataClient to finish
}

// requireValOrExit exits with an error message if `val` is empty.
func requireValOrExit(val string, errstr string) {
	if val == "" {
		fmt.Fprintf(os.Stderr, "%s\n", errstr)
		os.Exit(1)
	}
}
