# Build stage
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/incident-mcp-server ./cmd/incident-mcp-server

# Runtime stage — distroless, non-root, no shell
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /bin/incident-mcp-server /incident-mcp-server
EXPOSE 8081
ENTRYPOINT ["/incident-mcp-server"]
