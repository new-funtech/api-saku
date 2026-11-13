package services

import (
	"fmt"
	"strings"
	"time"
)

func notifLoanSubmittedApplicant() (title, body string) {
	return "Pengajuan pinjaman terkirim", "Pengajuan pinjaman berhasil dikirim."
}

func notifLoanSubmittedApprover(applicantName string) (title, body string) {
	return "Pengajuan pinjaman baru", fmt.Sprintf("Pengajuan pinjaman dari %s membutuhkan persetujuan Anda.", applicantName)
}

func notifLoanReviewRequired(applicantName string) (title, body string) {
	return "Pengajuan pinjaman baru membutuhkan persetujuan Anda", fmt.Sprintf("Pengajuan pinjaman dari %s membutuhkan persetujuan Anda.", applicantName)
}

func notifLoanApproved(levelName string) (title, body string) {
	return "Pengajuan pinjaman disetujui", fmt.Sprintf("Pengajuan pinjaman Anda telah disetujui pada tahap %s.", levelName)
}

func notifLoanCompleted() (title, body string) {
	return "Pengajuan pinjaman disetujui", "Pengajuan pinjaman Anda telah disetujui."
}

func notifLoanRejected(reason *string) (title, body string) {
	body = "Pengajuan pinjaman Anda ditolak."
	if reason != nil && *reason != "" {
		body = fmt.Sprintf("Pengajuan pinjaman Anda ditolak.\nAlasan: %s", *reason)
	}
	return "Pengajuan pinjaman ditolak", body
}

func notifLoanCancelled() (title, body string) {
	return "Pengajuan pinjaman dibatalkan", "Pengajuan pinjaman telah dibatalkan."
}

func notifOvertimeReviewRequired(applicantName string, date time.Time, hours float64) (title, body string) {
	return "Pengajuan lembur baru", fmt.Sprintf("%s mengajukan lembur %s (%.1f jam) dan membutuhkan persetujuan Anda.", applicantName, date.Format("02-01-2006"), hours)
}

func notifOvertimeDecision(approved bool, date time.Time, notes *string) (title, body string) {
	day := date.Format("02-01-2006")
	if approved {
		return "Pengajuan lembur disetujui", fmt.Sprintf("Pengajuan lembur Anda pada %s telah disetujui.", day)
	}
	body = fmt.Sprintf("Pengajuan lembur Anda pada %s ditolak.", day)
	if notes != nil && *notes != "" {
		body += "\nAlasan: " + *notes
	}
	return "Pengajuan lembur ditolak", body
}

// loanDetail is the line that identifies a loan in a notification: its number, amount and tenor.
func loanDetail(loanNumber string, amount float64, tenorMonths int) string {
	parts := make([]string, 0, 3)
	if loanNumber != "" {
		parts = append(parts, "No. "+loanNumber)
	}
	if amount > 0 {
		parts = append(parts, "Rp "+formatRupiah(amount))
	}
	if tenorMonths > 0 {
		parts = append(parts, fmt.Sprintf("%d bulan", tenorMonths))
	}
	return strings.Join(parts, " · ")
}

// withDetail puts a detail line right after the first sentence of a body, so the reason line
// (if any) stays last.
func withDetail(body, detail string) string {
	if strings.TrimSpace(detail) == "" {
		return body
	}
	first, rest, found := strings.Cut(body, "\n")
	if !found {
		return first + "\n" + detail
	}
	return first + "\n" + detail + "\n" + rest
}

func overtimeDetail(date time.Time, hours float64) string {
	return fmt.Sprintf("%s · %.1f jam", date.Format("02-01-2006"), hours)
}

var leaveTypeLabels = map[string]string{"leave": "Cuti", "permission": "Izin", "sick": "Sakit"}

func leaveDetail(kind string, start, end time.Time) string {
	label := leaveTypeLabels[kind]
	if label == "" {
		label = "Cuti/izin"
	}
	if start.Equal(end) {
		return fmt.Sprintf("%s · %s", label, start.Format("02-01-2006"))
	}
	return fmt.Sprintf("%s · %s s/d %s", label, start.Format("02-01-2006"), end.Format("02-01-2006"))
}

var correctionTypeLabels = map[string]string{"checkin": "jam masuk", "checkout": "jam pulang", "both": "jam masuk & pulang"}

func correctionDetail(kind string, date time.Time) string {
	label := correctionTypeLabels[kind]
	if label == "" {
		label = "absensi"
	}
	return fmt.Sprintf("Koreksi %s · %s", label, date.Format("02-01-2006"))
}

// decisionBody is the push text for a decided request: what it was, and the approver's note.
func decisionBody(detail string, notes *string) string {
	body := detail
	if notes != nil && strings.TrimSpace(*notes) != "" {
		body += "\nCatatan: " + strings.TrimSpace(*notes)
	}
	return body
}
