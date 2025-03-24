// Copyright (c) 2025 Neomantra Corp

package handlers

import (
	"database/sql"

	"github.com/NimbleMarkets/dbn-duckduck-goose/docs"
	"github.com/NimbleMarkets/dbn-duckduck-goose/middleware"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Global storage of database --
var (
	gDuckdbConn *sql.DB
)

// Register registers our custom Appplication with the passed gin.Engine
func Register(hostPort string, conn *sql.DB, r *gin.Engine, logger *zap.Logger) *gin.Engine {
	// Set module DuckDB connection
	gDuckdbConn = conn

	// Register swagger docs
	docs.SwaggerInfo.Host = hostPort
	r = RegisterOpenApiDocs(r)

	// An offering to the CORS guardians
	v1 := r.Group("/api/v1", middleware.CorsOptionHandlerWithVerbs("GET"))

	// Register our middleware suites
	RegisterSnapshotApi(v1)
	return r
}

// route setup broken out for testing - so we can replace middleware

// RegisterOpenApiDocs registers the /docs route for OpenAPI documentation
func RegisterOpenApiDocs(r *gin.Engine) *gin.Engine {
	g := r.Group("/docs")
	g.GET("/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	g.OPTIONS("/*any", middleware.CorsOptionHandlerWithVerbs("GET"))
	return r
}

// RegisterSnapshotApi registers the snapshot API routes
func RegisterSnapshotApi(r *gin.RouterGroup) *gin.RouterGroup {
	// last-trades
	g := r.Group("/last-trades")
	g.GET("/:dataset/:ticker", GetLastTradesByDatasetAndTicker)
	return r
}
