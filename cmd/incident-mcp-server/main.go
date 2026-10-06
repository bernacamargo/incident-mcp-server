package main

import (
	"log"
	"net/http"

	"github.com/bernacamargo/incident-mcp-server/internal/incidents"
	"github.com/bernacamargo/incident-mcp-server/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	svc := incidents.NewService(nil)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "incident-mcp-server",
		Version: "0.1.0",
	}, nil)
	tools.New(svc).Register(server)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil))

	addr := ":8081"
	log.Printf("incident-mcp-server serving MCP at http://localhost%s/mcp", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
