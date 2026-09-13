package main

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/server"

	"github.com/little-box-rin/stability-mcp/internal/client"
	"github.com/little-box-rin/stability-mcp/internal/config"
	"github.com/little-box-rin/stability-mcp/internal/tools"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("ERROR loading config: %v\n", err)
		return
	}
	fmt.Printf("Config OK: api_key=%s..., output_dir=%s\n",
		cfg.APIKey[:min(8, len(cfg.APIKey))], cfg.OutputDir)

	cli := client.NewClient(cfg.APIKey)
	s := server.NewMCPServer("stability-mcp", "0.4.0")

	if err := tools.RegisterAll(s, cfg, cli); err != nil {
		fmt.Printf("ERROR RegisterAll: %v\n", err)
		return
	}

	// Now try listing tools
	toolMap := s.ListTools()
	for name, st := range toolMap {
		b, err := json.Marshal(st.Tool)
		if err != nil {
			fmt.Printf("ERROR marshaling tool '%s': %v\n", name, err)
			ts := st.Tool.InputSchema
			for k, v := range ts.Properties {
				fmt.Printf("  '%s': type=%T val=%#v\n", k, v, v)
			}
			_ = ts.Required
			_ = ts.Type
		} else {
			fmt.Printf("Tool '%s' OK (%d bytes)\n", name, len(b))
		}
	}
}