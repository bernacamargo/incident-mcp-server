// Package incidents holds the incident domain and the policy-enforcing
// service behind the MCP tools. It intentionally imports nothing from the
// MCP SDK: the tool layer adapts this package, not the other way around.
package incidents

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Sentinel errors let callers — and LLM agents — branch on failure classes
// with errors.Is instead of string matching.
var (
	ErrNotFound     = errors.New("not found")
	ErrPolicyDenied = errors.New("policy denied")
	ErrInvalid      = errors.New("invalid argument")
)

type Status string

const (
	StatusOpen         Status = "Open"
	StatusAcknowledged Status = "Acknowledged"
	StatusResolved     Status = "Resolved"
)

func ValidStatus(s Status) bool {
	return s == StatusOpen || s == StatusAcknowledged || s == StatusResolved
}

type Severity string

const (
	Sev1 Severity = "SEV1"
	Sev2 Severity = "SEV2"
	Sev3 Severity = "SEV3"
)

func ValidSeverity(s Severity) bool {
	return s == Sev1 || s == Sev2 || s == Sev3
}

type Responder struct {
	ID   string
	Name string
	Team string
}

type OwningService struct {
	ID       string
	Name     string
	Team     string
	OnCallID string // Responder.ID currently on call
}

// OnCallShift is a flattened service+responder view for the listOnCall tool.
type OnCallShift struct {
	ServiceID   string
	ServiceName string
	Team        string
	OnCallName  string
}

type Incident struct {
	ID         string
	Title      string
	ServiceID  string
	Severity   Severity
	Status     Status
	AssignedTo string     // Responder.ID; empty until acknowledged
	ReportedAt time.Time
	AckedAt    *time.Time
	ResolvedAt *time.Time
}

type AuditEntry struct {
	At         time.Time
	Action     string // "reported" | "acknowledged" | "resolved"
	IncidentID string
	Detail     string
}

// Service is the policy-enforcing in-memory store. Every mutation validates
// input and policy here, so agents can never bypass business rules.
type Service struct {
	mu         sync.Mutex
	clock      func() time.Time
	responders map[string]Responder
	services   map[string]OwningService
	incidents  map[string]Incident
	audit      []AuditEntry
	seq        int
}

func NewService(clock func() time.Time) *Service {
	if clock == nil {
		clock = time.Now
	}
	s := &Service{
		clock:      clock,
		responders: map[string]Responder{},
		services:   map[string]OwningService{},
		incidents:  map[string]Incident{},
	}
	s.seed()
	return s
}

// ReportIncident opens a new incident against a known service. Policy:
// service must exist, severity must be valid.
func (s *Service) ReportIncident(serviceID, title string, severity Severity) (Incident, error) {
	if !ValidSeverity(severity) {
		return Incident{}, fmt.Errorf("%w: severity %q (want SEV1, SEV2 or SEV3)", ErrInvalid, severity)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	ow, ok := s.services[serviceID]
	if !ok {
		return Incident{}, fmt.Errorf("service %s: %w", serviceID, ErrNotFound)
	}
	s.seq++
	inc := Incident{
		ID:         fmt.Sprintf("INC-%04d", s.seq),
		Title:      title,
		ServiceID:  serviceID,
		Severity:   severity,
		Status:     StatusOpen,
		ReportedAt: s.clock(),
	}
	s.incidents[inc.ID] = inc
	s.audit = append(s.audit, AuditEntry{At: inc.ReportedAt, Action: "reported", IncidentID: inc.ID, Detail: fmt.Sprintf("%s on %s (on-call: %s)", severity, ow.Name, ow.OnCallID)})
	return inc, nil
}

// AcknowledgeIncident assigns the service's on-call responder. Policy:
// incident must exist and still be Open.
func (s *Service) AcknowledgeIncident(incidentID string) (Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inc, ok := s.incidents[incidentID]
	if !ok {
		return Incident{}, fmt.Errorf("incident %s: %w", incidentID, ErrNotFound)
	}
	if inc.Status != StatusOpen {
		return Incident{}, fmt.Errorf("%w: incident %s is %s, only Open incidents can be acknowledged", ErrPolicyDenied, incidentID, inc.Status)
	}
	ow := s.services[inc.ServiceID]
	now := s.clock()
	inc.Status = StatusAcknowledged
	inc.AssignedTo = ow.OnCallID
	inc.AckedAt = &now
	s.incidents[incidentID] = inc
	s.audit = append(s.audit, AuditEntry{At: now, Action: "acknowledged", IncidentID: incidentID, Detail: fmt.Sprintf("assigned to %s", ow.OnCallID)})
	return inc, nil
}

// ResolveIncident closes an incident. Policy: it must be Acknowledged first —
// resolving an unowned incident hides work nobody picked up.
func (s *Service) ResolveIncident(incidentID string) (Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inc, ok := s.incidents[incidentID]
	if !ok {
		return Incident{}, fmt.Errorf("incident %s: %w", incidentID, ErrNotFound)
	}
	if inc.Status != StatusAcknowledged {
		return Incident{}, fmt.Errorf("%w: incident %s is %s, it must be Acknowledged before resolution", ErrPolicyDenied, incidentID, inc.Status)
	}
	now := s.clock()
	inc.Status = StatusResolved
	inc.ResolvedAt = &now
	s.incidents[incidentID] = inc
	s.audit = append(s.audit, AuditEntry{At: now, Action: "resolved", IncidentID: incidentID, Detail: fmt.Sprintf("resolved by %s", inc.AssignedTo)})
	return inc, nil
}

// ListIncidents returns incidents sorted by ID, optionally filtered by status.
func (s *Service) ListIncidents(status Status) ([]Incident, error) {
	if status != "" && !ValidStatus(status) {
		return nil, fmt.Errorf("%w: status %q", ErrInvalid, status)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Incident, 0, len(s.incidents))
	for _, inc := range s.incidents {
		if status == "" || inc.Status == status {
			out = append(out, inc)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *Service) GetIncident(incidentID string) (Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inc, ok := s.incidents[incidentID]
	if !ok {
		return Incident{}, fmt.Errorf("incident %s: %w", incidentID, ErrNotFound)
	}
	return inc, nil
}

// ListOnCall returns who is on call for each service, sorted by service ID.
func (s *Service) ListOnCall() []OnCallShift {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]OnCallShift, 0, len(s.services))
	for _, ow := range s.services {
		name := "(none)"
		if r, ok := s.responders[ow.OnCallID]; ok {
			name = r.Name
		}
		out = append(out, OnCallShift{ServiceID: ow.ID, ServiceName: ow.Name, Team: ow.Team, OnCallName: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ServiceID < out[j].ServiceID })
	return out
}

// AuditEntries returns a copy of the audit trail, oldest first.
func (s *Service) AuditEntries() []AuditEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]AuditEntry, len(s.audit))
	copy(out, s.audit)
	return out
}

func (s *Service) seed() {
	s.responders["r-01"] = Responder{ID: "r-01", Name: "Priya Nair", Team: "Payments"}
	s.responders["r-02"] = Responder{ID: "r-02", Name: "Jonas Weber", Team: "Payments"}
	s.responders["r-03"] = Responder{ID: "r-03", Name: "Lucia Ferraz", Team: "Identity"}
	s.responders["r-04"] = Responder{ID: "r-04", Name: "Tomas Rocha", Team: "Search"}

	s.services["svc-id"] = OwningService{ID: "svc-id", Name: "identity-api", Team: "Identity", OnCallID: "r-03"}
	s.services["svc-pay"] = OwningService{ID: "svc-pay", Name: "payments-api", Team: "Payments", OnCallID: "r-01"}
	s.services["svc-srch"] = OwningService{ID: "svc-srch", Name: "search-api", Team: "Search", OnCallID: "r-04"}

	s.seq = 3
	t0 := s.clock()
	open := t0
	acked := t0
	resolved := t0
	s.incidents["INC-0001"] = Incident{ID: "INC-0001", Title: "Elevated 5xx on checkout payment intents", ServiceID: "svc-pay", Severity: Sev1, Status: StatusOpen, ReportedAt: open}
	s.incidents["INC-0002"] = Incident{ID: "INC-0002", Title: "Login latency p99 above SLO", ServiceID: "svc-id", Severity: Sev2, Status: StatusAcknowledged, AssignedTo: "r-03", ReportedAt: acked, AckedAt: &acked}
	s.incidents["INC-0003"] = Incident{ID: "INC-0003", Title: "Search index replication lag", ServiceID: "svc-srch", Severity: Sev3, Status: StatusResolved, AssignedTo: "r-04", ReportedAt: resolved, AckedAt: &resolved, ResolvedAt: &resolved}
}
