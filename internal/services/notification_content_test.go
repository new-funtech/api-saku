package services

import (
	"strings"
	"testing"
)

// This repo has no existing test infrastructure at all (no test DB/fixtures,
// no _test.go files anywhere before this one). The recipient-resolution and
// idempotency logic in notification_service.go genuinely needs a live
// Postgres to exercise meaningfully and is out of scope to fake here — but
// the notification content templates are pure functions with no
// dependencies, so they're covered directly.

func TestNotifLoanRejected_IncludesReasonVerbatim(t *testing.T) {
	reason := "Dokumen KTP tidak jelas"
	_, body := notifLoanRejected(&reason)
	if !strings.Contains(body, reason) {
		t.Fatalf("expected body to contain the reason verbatim, got: %q", body)
	}
}

func TestNotifLoanRejected_NilReasonOmitsReasonLine(t *testing.T) {
	_, body := notifLoanRejected(nil)
	if strings.Contains(body, "Alasan:") {
		t.Fatalf("expected no 'Alasan:' line when reason is nil, got: %q", body)
	}
}

func TestNotifLoanApproved_DoesNotLeakAmountOrTenor(t *testing.T) {
	// Security requirement: notification text must never carry loan
	// amount/tenor. A crude but effective regression guard: the word "Rp"
	// (the only way amounts are ever formatted elsewhere in this codebase,
	// see notifier/notifier.go's formatRupiah) must never appear here.
	_, body := notifLoanApproved("Admin Perusahaan")
	if strings.Contains(body, "Rp") {
		t.Fatalf("expected no amount in approval notification body, got: %q", body)
	}
}

func TestNotifLoanCompleted_IsFinalMessage(t *testing.T) {
	title, body := notifLoanCompleted()
	if title == "" || body == "" {
		t.Fatal("expected non-empty title and body for the final approval notification")
	}
}

func TestNotifLoanSubmittedApprover_MentionsApplicantName(t *testing.T) {
	_, body := notifLoanSubmittedApprover("Budi Santoso")
	if !strings.Contains(body, "Budi Santoso") {
		t.Fatalf("expected applicant name in review-required body, got: %q", body)
	}
}

func TestLoanDetailIdentifiesTheLoan(t *testing.T) {
	if got := loanDetail("PJ-2026-0007", 5000000, 12); got != "No. PJ-2026-0007 · Rp 5.000.000 · 12 bulan" {
		t.Fatalf("detail = %q", got)
	}
	if got := loanDetail("", 0, 0); got != "" {
		t.Fatalf("empty detail = %q", got)
	}
}

func TestWithDetailGoesAfterTheFirstSentence(t *testing.T) {
	got := withDetail("Pengajuan ditolak.\nAlasan: tidak lengkap", "No. PJ-1")
	if got != "Pengajuan ditolak.\nNo. PJ-1\nAlasan: tidak lengkap" {
		t.Fatalf("body = %q", got)
	}
	if got := withDetail("Disetujui.", "No. PJ-1"); got != "Disetujui.\nNo. PJ-1" {
		t.Fatalf("body = %q", got)
	}
	if got := withDetail("Disetujui.", " "); got != "Disetujui." {
		t.Fatalf("body = %q", got)
	}
}
