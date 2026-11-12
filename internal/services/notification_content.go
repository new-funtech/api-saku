package services

import (
	"fmt"
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
