package incidents

import (
	"errors"
	"testing"
	"time"
)

func fixedClock() (func() time.Time, time.Time) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return now }, now
}

func newTestService() *Service {
	clock, _ := fixedClock()
	return NewService(clock)
}

func TestReportIncident(t *testing.T) {
	svc := newTestService()

	inc, err := svc.ReportIncident("svc-pay", "Card auth timeouts in EU region", Sev1)
	if err != nil {
		t.Fatalf("ReportIncident() error = %v", err)
	}
	if inc.ID != "INC-0004" || inc.Status != StatusOpen {
		t.Errorf("got ID=%s Status=%s, want INC-0004 Open", inc.ID, inc.Status)
	}

	audit := svc.AuditEntries()
	if len(audit) != 1 || audit[0].Action != "reported" {
		t.Errorf("audit = %+v, want one reported entry", audit)
	}
}

func TestReportIncidentUnknownService(t *testing.T) {
	svc := newTestService()

	_, err := svc.ReportIncident("svc-nope", "Ghost incident", Sev2)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestReportIncidentInvalidSeverity(t *testing.T) {
	svc := newTestService()

	_, err := svc.ReportIncident("svc-pay", "Bad severity", Severity("SEV0"))
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("error = %v, want ErrInvalid", err)
	}
}

func TestAcknowledgeIncidentAssignsOnCall(t *testing.T) {
	svc := newTestService()

	inc, err := svc.AcknowledgeIncident("INC-0001")
	if err != nil {
		t.Fatalf("AcknowledgeIncident() error = %v", err)
	}
	if inc.AssignedTo != "r-01" || inc.Status != StatusAcknowledged || inc.AckedAt == nil {
		t.Errorf("got %+v, want assigned to r-01 Acknowledged with AckedAt", inc)
	}
}

func TestAcknowledgePolicyDenials(t *testing.T) {
	svc := newTestService()

	// Already acknowledged
	if _, err := svc.AcknowledgeIncident("INC-0002"); !errors.Is(err, ErrPolicyDenied) {
		t.Errorf("error = %v, want ErrPolicyDenied", err)
	}
	// Unknown id
	if _, err := svc.AcknowledgeIncident("INC-9999"); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestResolveRequiresAcknowledgement(t *testing.T) {
	svc := newTestService()

	// Open incidents cannot be resolved
	if _, err := svc.ResolveIncident("INC-0001"); !errors.Is(err, ErrPolicyDenied) {
		t.Errorf("error = %v, want ErrPolicyDenied", err)
	}

	// Happy path
	inc, err := svc.ResolveIncident("INC-0002")
	if err != nil {
		t.Fatalf("ResolveIncident() error = %v", err)
	}
	if inc.Status != StatusResolved || inc.ResolvedAt == nil {
		t.Errorf("got %+v, want Resolved with ResolvedAt", inc)
	}
}

func TestListIncidentsFilterAndOrder(t *testing.T) {
	svc := newTestService()

	open, err := svc.ListIncidents(StatusOpen)
	if err != nil {
		t.Fatalf("ListIncidents() error = %v", err)
	}
	if len(open) != 1 || open[0].ID != "INC-0001" {
		t.Errorf("open = %+v, want only INC-0001", open)
	}

	all, err := svc.ListIncidents("")
	if err != nil {
		t.Fatalf("ListIncidents() error = %v", err)
	}
	if len(all) != 3 || all[0].ID > all[1].ID || all[1].ID > all[2].ID {
		t.Errorf("all = %+v, want 3 incidents sorted by ID", all)
	}

	if _, err := svc.ListIncidents(Status("Bogus")); !errors.Is(err, ErrInvalid) {
		t.Errorf("error = %v, want ErrInvalid", err)
	}
}

func TestListOnCall(t *testing.T) {
	svc := newTestService()

	shifts := svc.ListOnCall()
	if len(shifts) != 3 {
		t.Fatalf("ListOnCall() = %+v, want 3 shifts", shifts)
	}
	if shifts[0].ServiceID != "svc-id" || shifts[0].OnCallName != "Lucia Ferraz" {
		t.Errorf("first shift = %+v, want svc-id with Lucia Ferraz", shifts[0])
	}
}

func TestAuditEntriesOldestFirst(t *testing.T) {
	svc := newTestService()

	if _, err := svc.ResolveIncident("INC-0002"); err != nil {
		t.Fatalf("ResolveIncident() error = %v", err)
	}
	if _, err := svc.ReportIncident("svc-pay", "Follow-up check", Sev3); err != nil {
		t.Fatalf("ReportIncident() error = %v", err)
	}

	audit := svc.AuditEntries()
	if len(audit) != 2 || audit[0].Action != "resolved" || audit[1].Action != "reported" {
		t.Errorf("audit = %+v, want resolved then reported", audit)
	}
}
