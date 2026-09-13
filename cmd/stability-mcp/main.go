package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/server"

	"github.com/little-box-rin/stability-mcp/internal/client"
	"github.com/little-box-rin/stability-mcp/internal/config"
	"github.com/little-box-rin/stability-mcp/internal/tools"
)

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
		log.Fatalf("Failed to load config: %v", err)
	}
	if outputDir != "" {
		cfg.OutputDir = outputDir
	}
	if concurrency > 0 {
		cfg.Concurrency = concurrency
	}

	cli := client.NewClient(cfg.APIKey)

	mcpServer := server.NewMCPServer("stability-mcp", "0.3.0")

	if err := tools.RegisterAll(mcpServer, cfg, cli); err != nil {
		log.Fatalf("Failed to register tools: %v", err)
	}

	if sseMode {
		log.Printf("Starting SSE server on %s", addr)

		// Create SSE transport
		sseServer := server.NewSSEServer(mcpServer, server.WithBaseURL(fmt.Sprintf("http://%s", addr)))

		// Handle shutdown via signal
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

		httpServer := &http.Server{
			Addr: addr,
			Handler: sseServer,
		}

		go func() {
			<-stop
			log.Println("Shutting down SSE server...")
			httpServer.Close()
		}()

		if devTLS {
			log.Fatal(httpServer.ListenAndServeTLS("cert.pem", "key.pem"))
		} else {
			log.Fatal(httpServer.ListenAndServe())
		}
	} else {
		if err := server.ServeStdio(mcpServer); err != nil {
			log.Fatalf("Stdio server error: %v", err)
		}
	}
}