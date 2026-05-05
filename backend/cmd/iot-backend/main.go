package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"iotsmart/backend/internal/api"
	"iotsmart/backend/internal/config"
	"iotsmart/backend/internal/connectors/mqtt"
	"iotsmart/backend/internal/db"
	"iotsmart/backend/internal/ingest"
	"iotsmart/backend/internal/logger"
	"iotsmart/backend/internal/scheduler"
	"iotsmart/backend/internal/security"
	"iotsmart/backend/internal/websocket"
)

func main() {
	configPath := flag.String("config", filepath.Join("configs", "app.yaml"), "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	appLogger, closer, err := logger.New(cfg.Logging)
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer closer.Close()

	database, err := db.Open(cfg.Database)
	if err != nil {
		appLogger.Fatalf("open database: %v", err)
	}
	defer closeDB(database, appLogger.Logger)

	if err := db.Migrate(database); err != nil {
		appLogger.Fatalf("apply migrations: %v", err)
	}

	repo := db.NewRepository(database)
	hub := websocket.NewHub(appLogger.Logger)
	ingestor := ingest.NewService(repo, hub, appLogger.Logger)
	authService := security.NewService(cfg.Security, repo, appLogger.Logger)
	simulator := scheduler.NewSimulator(cfg, repo, hub, appLogger.Logger)
	mqttConnector := mqtt.New(cfg.MQTT, ingestor, appLogger.Logger)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mqttConnector.Start(rootCtx); err != nil {
		appLogger.Printf("start mqtt connector: %v", err)
	}

	if cfg.Simulator.Enabled {
		if err := simulator.Start(rootCtx); err != nil {
			appLogger.Fatalf("start simulator: %v", err)
		}
	}

	server := api.NewServer(cfg, repo, hub, database, appLogger, simulator, mqttConnector, ingestor, authService)
	httpServer := &http.Server{
		Addr:              net.JoinHostPort(cfg.App.BindHost, cfg.App.PortString()),
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		appLogger.Printf("iot backend listening on http://%s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Fatalf("http server: %v", err)
		}
	}()

	waitForShutdown(httpServer, simulator, mqttConnector, appLogger.Logger)
}

func waitForShutdown(httpServer *http.Server, simulator *scheduler.Simulator, mqttConnector *mqtt.Connector, appLogger *log.Logger) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals

	appLogger.Println("shutdown signal received")
	simulator.Stop()
	mqttConnector.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		appLogger.Printf("graceful shutdown error: %v", err)
	}
}

func closeDB(database *sql.DB, appLogger *log.Logger) {
	if err := database.Close(); err != nil {
		appLogger.Printf("close database: %v", err)
	}
}
