package services

import "fmt"

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
