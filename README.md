# incident-mcp-server

[![CI](https://github.com/bernacamargo/incident-mcp-server/actions/workflows/ci.yml/badge.svg)](https://github.com/bernacamargo/incident-mcp-server/actions/workflows/ci.yml)

A **Model Context Protocol (MCP) server** for on-call incident response — built in **Go** with the official [go-sdk](https://github.com/modelcontextprotocol/go-sdk).

AI agents get a typed, policy-enforced toolbox for the incident lifecycle — report, acknowledge (auto-assigns the on-call responder), resolve — plus service/on-call lookups and a full audit trail. Business rules live server-side: an agent cannot resolve an unacknowledged incident or page a ghost service.

> Portfolio project. Data is in-memory seed data; the value is the production-grade MCP integration, not persistence.

**The Kotlin twin:** [iam-mcp-server](https://github.com/bernacamargo/iam-mcp-server) — the same protocol in Spring Boot 4 + Spring AI, applied to identity governance. Two ecosystems, one discipline.

## Tools exposed

| Tool | Description |
|---|---|
| `listIncidents` | Incidents sorted by ID, optional status filter (`Open`, `Acknowledged`, `Resolved`) |
| `getIncident` | One incident with full lifecycle timestamps |
| `listOnCall` | Who is on call for each service |
| `reportIncident` | Open an incident against a known service (`SEV1`–`SEV3`) |
| `acknowledgeIncident` | Acknowledge an `Open` incident — auto-assigns the on-call responder |
| `resolveIncident` | Resolve an incident that has been acknowledged |
| `listAuditEntries` | Audit trail of every lifecycle transition |

## Stack

- **Go 1.27**, official MCP **go-sdk v1.8** (`AddTool` generics → schemas inferred from typed structs)
- Streamable HTTP transport at `/mcp`
- Table-driven tests with a fixed clock; `golangci-lint` in CI

## Run it

```bash
go run ./cmd/incident-mcp-server
```

The MCP endpoint is served over streamable HTTP at `http://localhost:8081/mcp`.

Point any MCP client at it, e.g. Claude Code:

```json
{
  "mcpServers": {
    "incidents": {
      "type": "http",
      "url": "http://localhost:8081/mcp"
    }
  }
}
```

Then try: *"What's on fire right now?"*, *"Acknowledge the payments incident"* — or *"Resolve INC-0001"* and watch the policy engine refuse because nobody picked it up.

## Architecture

```
MCP client (Claude, IDE agent, ...)
        │  streamable HTTP (/mcp)
        ▼
┌────────────────────────────────────┐
│  go-sdk: schema inference, I/O     │  typed In/Out per tool
│  ────────────────────────────────  │
│  internal/tools                    │  MCP adapters (only MCP-aware layer)
│  ────────────────────────────────  │
│  internal/incidents                │  policy + audit; zero MCP imports
│  ────────────────────────────────  │
│  domain: Incident, OwningService   │  plain structs, sentinel errors
└────────────────────────────────────┘
```

## Design notes

**Sentinel errors are the error API.** `ErrNotFound`, `ErrPolicyDenied`, and `ErrInvalid` are wrapped with `%w`, so callers branch with `errors.Is`. An agent that hears *"policy denied: incident INC-0001 is Resolved"* knows the difference between a bad ID and a rule it can't break — and that distinction comes for free from Go error wrapping.

**The service package imports no MCP code.** Everything about the incident domain — policy, audit, invariants — lives in `internal/incidents` and is testable without any protocol in the loop. `internal/tools` is a thin adapter; swapping MCP for anything else is one package's problem.

**Schemas from types, not maps.** `mcp.AddTool[In, Out]` infers JSON schemas from typed args structs, so a compile error is a schema error caught before runtime.

**Policies mirror real on-call discipline:** acknowledge-before-resolve, auto-assignment to the on-call responder, severity validation at the door, and an audit entry for every transition.

## Container

```bash
docker build -t incident-mcp-server .
docker run -p 8081:8081 incident-mcp-server
```

Multi-stage build: Go compiles a static binary, runtime is a distroless non-root image with no shell inside.

## Roadmap

- [x] **M1** — skeleton, seven tools, table-driven tests, CI (test + lint)
- [x] **M3** — container image (distroless), release automation, design notes
- [ ] **M2** — Postgres persistence, paging tool with escalation policy
