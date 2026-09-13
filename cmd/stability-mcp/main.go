package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/little-box-rin/stability-mcp/internal/client"
	"github.com/little-box-rin/stability-mcp/internal/config"
	"github.com/little-box-rin/stability-mcp/internal/tools"
)

const version = "0.4.0"

var startTime = time.Now()

func main() {
	var (
		sseMode     bool
		addr        string
		devTLS      bool
		outputDir   string
		concurrency int
	)
	flag.BoolVar(&sseMode, "sse", false, "Run in SSE mode instead of stdio")
	flag.StringVar(&addr, "addr", ":8080", "Listen address for SSE mode")
	flag.BoolVar(&devTLS, "dev-tls", false, "Enable self-signed TLS for SSE mode")
	flag.StringVar(&outputDir, "output-dir", "", "Override output directory")
	flag.IntVar(&concurrency, "concurrency", 10, "Default batch concurrency")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Set log level from config.
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	if outputDir != "" {
		cfg.OutputDir = outputDir
	}
	if concurrency > 0 {
		cfg.Concurrency = concurrency
	}

	cli := client.NewClient(cfg.APIKey)
	cli.RetryConfig.MaxRetries = cfg.MaxRetries

	mcpServer := server.NewMCPServer("stability-mcp", version)

	if err := tools.RegisterAll(mcpServer, cfg, cli); err != nil {
		slog.Error("Failed to register tools", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting stability-mcp", "version", version, "output_dir", cfg.OutputDir,
		"concurrency", cfg.Concurrency, "rate_limit", cfg.RateLimit, "max_retries", cfg.MaxRetries)

	if sseMode {
		slog.Info("Starting SSE server", "addr", addr)

		// Create SSE transport
		sseServer := server.NewSSEServer(mcpServer, server.WithBaseURL(fmt.Sprintf("http://%s", addr)))

		// Handle shutdown via signal
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

		mux := http.NewServeMux()
		mux.Handle("/", sseServer)
		mux.HandleFunc("/health", healthHandler)

		httpServer := &http.Server{
			Addr:    addr,
			Handler: mux,
		}

		go func() {
			<-stop
			slog.Info("Shutting down SSE server...")
			httpServer.Close()
		}()

		var serveErr error
		if devTLS {
			serveErr = httpServer.ListenAndServeTLS("cert.pem", "key.pem")
		} else {
			serveErr = httpServer.ListenAndServe()
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			slog.Error("Server error", "error", serveErr)
			os.Exit(1)
		}
	} else {
		if err := server.ServeStdio(mcpServer); err != nil {
			slog.Error("Stdio server error", "error", err)
			os.Exit(1)
		}
	}
}

// healthHandler reports the server health status as JSON.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "ok",
		"version":        version,
		"uptime_seconds": int(time.Since(startTime).Seconds()),
	})
}