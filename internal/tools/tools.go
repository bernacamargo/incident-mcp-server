// Package tools adapts the incidents service into MCP tools. It is the only
// layer that knows about the MCP SDK.
package tools

import (
	"context"

	"github.com/bernacamargo/incident-mcp-server/internal/incidents"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Handler struct {
	svc *incidents.Service
}

func New(svc *incidents.Service) *Handler {
	return &Handler{svc: svc}
}

// Register wires every tool onto the MCP server. Input and output schemas are
// inferred from the typed args/results; jsonschema tags carry the descriptions.
func (h *Handler) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "listIncidents",
		Description: "List incidents sorted by ID, optionally filtered by status (Open Acknowledged or Resolved)",
	}, h.listIncidents)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "getIncident",
		Description: "Get one incident by ID with its full lifecycle timestamps",
	}, h.getIncident)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "listOnCall",
		Description: "Show who is on call for each service",
	}, h.listOnCall)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reportIncident",
		Description: "Open a new incident against a known service with severity SEV1 SEV2 or SEV3",
	}, h.reportIncident)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "acknowledgeIncident",
		Description: "Acknowledge an Open incident, assigning the service on-call responder",
	}, h.acknowledgeIncident)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "resolveIncident",
		Description: "Resolve an incident that has been acknowledged",
	}, h.resolveIncident)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "listAuditEntries",
		Description: "Audit trail of every incident report acknowledgement and resolution, oldest first",
	}, h.listAuditEntries)
}

type listIncidentsIn struct {
	Status string `json:"status,omitempty" jsonschema:"optional status filter: Open Acknowledged or Resolved"`
}

func (h *Handler) listIncidents(ctx context.Context, req *mcp.CallToolRequest, in listIncidentsIn) (*mcp.CallToolResult, []incidents.Incident, error) {
	incidentsList, err := h.svc.ListIncidents(incidents.Status(in.Status))
	if err != nil {
		return nil, nil, err
	}
	return nil, incidentsList, nil
}

type getIncidentIn struct {
	IncidentID string `json:"incidentId" jsonschema:"incident ID e.g. INC-0001"`
}

func (h *Handler) getIncident(ctx context.Context, req *mcp.CallToolRequest, in getIncidentIn) (*mcp.CallToolResult, incidents.Incident, error) {
	inc, err := h.svc.GetIncident(in.IncidentID)
	if err != nil {
		return nil, incidents.Incident{}, err
	}
	return nil, inc, nil
}

type emptyIn struct{}

func (h *Handler) listOnCall(ctx context.Context, req *mcp.CallToolRequest, in emptyIn) (*mcp.CallToolResult, []incidents.OnCallShift, error) {
	return nil, h.svc.ListOnCall(), nil
}

type reportIncidentIn struct {
	ServiceID string `json:"serviceId" jsonschema:"owning service ID e.g. svc-pay"`
	Title     string `json:"title" jsonschema:"short human-readable incident summary"`
	Severity  string `json:"severity" jsonschema:"SEV1 SEV2 or SEV3"`
}

func (h *Handler) reportIncident(ctx context.Context, req *mcp.CallToolRequest, in reportIncidentIn) (*mcp.CallToolResult, incidents.Incident, error) {
	inc, err := h.svc.ReportIncident(in.ServiceID, in.Title, incidents.Severity(in.Severity))
	if err != nil {
		return nil, incidents.Incident{}, err
	}
	return nil, inc, nil
}

type lifecycleIn struct {
	IncidentID string `json:"incidentId" jsonschema:"incident ID e.g. INC-0001"`
}

func (h *Handler) acknowledgeIncident(ctx context.Context, req *mcp.CallToolRequest, in lifecycleIn) (*mcp.CallToolResult, incidents.Incident, error) {
	inc, err := h.svc.AcknowledgeIncident(in.IncidentID)
	if err != nil {
		return nil, incidents.Incident{}, err
	}
	return nil, inc, nil
}

func (h *Handler) resolveIncident(ctx context.Context, req *mcp.CallToolRequest, in lifecycleIn) (*mcp.CallToolResult, incidents.Incident, error) {
	inc, err := h.svc.ResolveIncident(in.IncidentID)
	if err != nil {
		return nil, incidents.Incident{}, err
	}
	return nil, inc, nil
}

func (h *Handler) listAuditEntries(ctx context.Context, req *mcp.CallToolRequest, in emptyIn) (*mcp.CallToolResult, []incidents.AuditEntry, error) {
	return nil, h.svc.AuditEntries(), nil
}
