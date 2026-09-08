package notifier

import (
	"context"
	"fmt"
	"strings"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
)

func (n *Notifier) LoanSubmitted(ctx context.Context, loan *model.Loan) {
	if n == nil || loan == nil {
		return
	}
	to := n.adminCompanyEmails(ctx, loan.BujpID)
	if len(to) == 0 {
		return
	}
	fullName := personnelName(loan.Personnel)
	body := paragraph(fmt.Sprintf("Pengajuan pinjaman baru dari <strong>%s</strong> menunggu peninjauan Anda sebagai admin perusahaan.", htmlEscape(fullName)))
	body += loanDetailsCard(loan)
	body += noticeBox(toneInfo, "Tindakan Diperlukan",
		"Silakan login ke aplikasi SAKU untuk meninjau dan menyetujui atau menolak pengajuan ini.")

	html := wrapEmail(
		"Pengajuan Pinjaman Baru",
		fmt.Sprintf("Permohonan dari %s menunggu persetujuan perusahaan.", fullName),
		"Menunggu Persetujuan", toneInfo,
		fmt.Sprintf("Pengajuan pinjaman baru dari %s — menunggu persetujuan perusahaan.", fullName),
		body,
	)
	subject := fmt.Sprintf("[SAKU] Pengajuan Pinjaman Baru — %s", loan.LoanNumber)
	n.dispatch(ctx, to, subject, html)
}

func (n *Notifier) LoanBujpDecision(ctx context.Context, loan *model.Loan, approved bool, notes *string) {
	if n == nil || loan == nil {
		return
	}
	fullName := personnelName(loan.Personnel)
	noteStr := derefStr(notes)

	if approved {
		to := append(n.adminPusatEmails(ctx), personnelEmail(loan.Personnel)...)
		body := paragraph(fmt.Sprintf("Pengajuan pinjaman dari <strong>%s</strong> telah <strong style=\"color:#047857\">disetujui oleh admin perusahaan</strong> dan dilanjutkan ke admin pusat untuk peninjauan akhir.", htmlEscape(fullName)))
		body += loanDetailsCard(loan)
		if noteStr != "" {
			body += noticeBox(toneInfo, "Catatan Perusahaan", htmlEscape(noteStr))
		}
		body += noticeBox(toneInfo, "Langkah Selanjutnya",
			"Admin pusat akan meninjau pengajuan ini dalam waktu dekat. Anda akan menerima email konfirmasi setelah keputusan akhir dibuat.")
		html := wrapEmail("Pinjaman Disetujui Perusahaan",
			fmt.Sprintf("Pengajuan %s diteruskan ke admin pusat.", loan.LoanNumber),
			"Disetujui Perusahaan", toneSuccess,
			fmt.Sprintf("Pinjaman %s disetujui perusahaan — menunggu admin pusat.", loan.LoanNumber),
			body)
		n.dispatch(ctx, to, fmt.Sprintf("[SAKU] Pinjaman Disetujui Perusahaan — %s", loan.LoanNumber), html)
		return
	}

	// Rejected by company admin → notify the applicant only.
	to := personnelEmail(loan.Personnel)
	body := paragraph("Mohon maaf, pengajuan pinjaman Anda telah <strong style=\"color:#b91c1c\">ditolak oleh admin perusahaan</strong>.")
	body += loanDetailsCard(loan)
	if noteStr != "" {
		body += noticeBox(toneDanger, "Alasan Penolakan", htmlEscape(noteStr))
	}
	body += noticeBox(toneInfo, "Butuh Bantuan?",
		"Hubungi admin perusahaan Anda untuk informasi lebih lanjut atau ajukan kembali setelah memperbaiki dokumen yang diperlukan.")
	html := wrapEmail("Pinjaman Ditolak",
		"Pengajuan tidak dapat dilanjutkan oleh admin perusahaan.",
		"Ditolak", toneDanger,
		fmt.Sprintf("Pinjaman %s ditolak oleh admin perusahaan.", loan.LoanNumber),
		body)
	n.dispatch(ctx, to, fmt.Sprintf("[SAKU] Pinjaman Ditolak — %s", loan.LoanNumber), html)
}

func (n *Notifier) LoanPusatDecision(ctx context.Context, loan *model.Loan, approved bool, notes *string) {
	if n == nil || loan == nil {
		return
	}
	fullName := personnelName(loan.Personnel)
	noteStr := derefStr(notes)

	if approved {
		to := n.adminBprksEmails(ctx)
		body := paragraph(fmt.Sprintf("Pengajuan pinjaman dari <strong>%s</strong> telah <strong style=\"color:#047857\">disetujui oleh admin pusat</strong> dan dilanjutkan ke BPRKS untuk peninjauan akhir.", htmlEscape(fullName)))
		body += loanDetailsCard(loan)
		if noteStr != "" {
			body += noticeBox(toneInfo, "Catatan Admin Pusat", htmlEscape(noteStr))
		}
		body += noticeBox(toneInfo, "Langkah Selanjutnya",
			"BPRKS akan melakukan peninjauan akhir sebelum pengajuan diteruskan ke pemohon untuk konfirmasi.")
		html := wrapEmail("Pinjaman Disetujui Admin Pusat",
			fmt.Sprintf("Pengajuan %s diteruskan ke BPRKS.", loan.LoanNumber),
			"Disetujui Pusat", toneSuccess,
			fmt.Sprintf("Pinjaman %s disetujui admin pusat — menunggu BPRKS.", loan.LoanNumber),
			body)
		n.dispatch(ctx, to, fmt.Sprintf("[SAKU] Pinjaman Disetujui Admin Pusat — %s", loan.LoanNumber), html)
		return
	}

	to := append(n.adminCompanyEmails(ctx, loan.BujpID), personnelEmail(loan.Personnel)...)
	body := paragraph(fmt.Sprintf("Pengajuan pinjaman dari <strong>%s</strong> telah <strong style=\"color:#b91c1c\">ditolak oleh admin pusat</strong>.", htmlEscape(fullName)))
	body += loanDetailsCard(loan)
	if noteStr != "" {
		body += noticeBox(toneDanger, "Alasan Penolakan", htmlEscape(noteStr))
	}
	body += noticeBox(toneInfo, "Informasi",
		"Pemohon dapat menghubungi admin perusahaan untuk informasi lebih lanjut atau mengajukan kembali sesuai prosedur.")
	html := wrapEmail("Pinjaman Ditolak Admin Pusat",
		"Pengajuan tidak dapat dilanjutkan oleh admin pusat.",
		"Ditolak Pusat", toneDanger,
		fmt.Sprintf("Pinjaman %s ditolak oleh admin pusat.", loan.LoanNumber),
		body)
	n.dispatch(ctx, to, fmt.Sprintf("[SAKU] Pinjaman Ditolak Admin Pusat — %s", loan.LoanNumber), html)
}

func (n *Notifier) LoanBprksDecision(ctx context.Context, loan *model.Loan, approved bool, notes *string) {
	if n == nil || loan == nil {
		return
	}
	fullName := personnelName(loan.Personnel)
	noteStr := derefStr(notes)
	to := append(n.adminCompanyEmails(ctx, loan.BujpID), n.adminPusatEmails(ctx)...)
	to = append(to, personnelEmail(loan.Personnel)...)

	if approved {
		body := paragraph(fmt.Sprintf("Pengajuan pinjaman dari <strong>%s</strong> telah <strong style=\"color:#047857\">disetujui oleh BPRKS</strong>.", htmlEscape(fullName)))
		body += loanDetailsCard(loan)
		if noteStr != "" {
			body += noticeBox(toneInfo, "Catatan BPRKS", htmlEscape(noteStr))
		}
		body += noticeBox(toneWarn, "Konfirmasi Pemohon Diperlukan",
			"Pemohon perlu melakukan konfirmasi penerimaan dana melalui aplikasi SAKU sebelum proses pencairan dapat dilanjutkan.")
		html := wrapEmail("Pinjaman Disetujui BPRKS",
			fmt.Sprintf("Pengajuan %s menunggu konfirmasi pemohon.", loan.LoanNumber),
			"Disetujui BPRKS", toneSuccess,
			fmt.Sprintf("Pinjaman %s disetujui BPRKS.", loan.LoanNumber),
			body)
		n.dispatch(ctx, to, fmt.Sprintf("[SAKU] Pinjaman Disetujui BPRKS — %s", loan.LoanNumber), html)
		return
	}

	body := paragraph(fmt.Sprintf("Pengajuan pinjaman dari <strong>%s</strong> telah <strong style=\"color:#b91c1c\">ditolak oleh BPRKS</strong>.", htmlEscape(fullName)))
	body += loanDetailsCard(loan)
	if noteStr != "" {
		body += noticeBox(toneDanger, "Alasan Penolakan", htmlEscape(noteStr))
	}
	body += noticeBox(toneInfo, "Informasi",
		"Pemohon dapat menghubungi admin perusahaan untuk informasi lebih lanjut atau mengajukan kembali sesuai prosedur.")
	html := wrapEmail("Pinjaman Ditolak BPRKS",
		"Pengajuan tidak dapat dilanjutkan oleh BPRKS.",
		"Ditolak BPRKS", toneDanger,
		fmt.Sprintf("Pinjaman %s ditolak oleh BPRKS.", loan.LoanNumber),
		body)
	n.dispatch(ctx, to, fmt.Sprintf("[SAKU] Pinjaman Ditolak BPRKS — %s", loan.LoanNumber), html)
}

func (n *Notifier) LoanUserConfirmed(ctx context.Context, loan *model.Loan, accepted bool, reason *string) {
	if n == nil || loan == nil {
		return
	}
	fullName := personnelName(loan.Personnel)
	to := append(n.adminCompanyEmails(ctx, loan.BujpID), n.adminPusatEmails(ctx)...)
	noteStr := derefStr(reason)

	if accepted {
		body := paragraph(fmt.Sprintf("<strong>%s</strong> telah <strong style=\"color:#047857\">menerima penawaran pinjaman</strong> dan proses pencairan akan dilanjutkan.", htmlEscape(fullName)))
		body += loanDetailsCard(loan)
		body += noticeBox(toneSuccess, "Status Saat Ini",
			"Pinjaman telah masuk ke tahap pencairan. Tim keuangan akan memproses transfer dana sesuai prosedur.")
		html := wrapEmail("Pemohon Menerima Pinjaman",
			fmt.Sprintf("%s telah mengonfirmasi penerimaan pinjaman.", fullName),
			"Diterima Pemohon", toneSuccess,
			fmt.Sprintf("Pemohon %s menerima pinjaman %s.", fullName, loan.LoanNumber),
			body)
		n.dispatch(ctx, to, fmt.Sprintf("[SAKU] Pemohon Menerima Pinjaman — %s", loan.LoanNumber), html)
		return
	}

	body := paragraph(fmt.Sprintf("<strong>%s</strong> telah <strong style=\"color:#b91c1c\">menolak penawaran pinjaman</strong>. Pengajuan dibatalkan secara otomatis.", htmlEscape(fullName)))
	body += loanDetailsCard(loan)
	if noteStr != "" {
		body += noticeBox(toneDanger, "Alasan Penolakan", htmlEscape(noteStr))
	}
	html := wrapEmail("Pemohon Menolak Pinjaman",
		fmt.Sprintf("%s membatalkan penerimaan pinjaman.", fullName),
		"Ditolak Pemohon", toneDanger,
		fmt.Sprintf("Pemohon %s menolak pinjaman %s.", fullName, loan.LoanNumber),
		body)
	n.dispatch(ctx, to, fmt.Sprintf("[SAKU] Pemohon Menolak Pinjaman — %s", loan.LoanNumber), html)
}

func loanDetailsCard(loan *model.Loan) string {
	if loan == nil {
		return ""
	}
	amount := loan.LoanAmount
	if loan.ApprovedAmount != nil && *loan.ApprovedAmount > 0 {
		amount = *loan.ApprovedAmount
	}
	tenor := loan.TenorMonths
	if loan.ApprovedTenor != nil && *loan.ApprovedTenor > 0 {
		tenor = *loan.ApprovedTenor
	}
	rows := [][2]string{
		{"Nomor Pengajuan", fmt.Sprintf("<span style=\"font-family:'SF Mono','Cascadia Code','Courier New',monospace\">%s</span>", htmlEscape(loan.LoanNumber))},
		{"Pemohon", htmlEscape(personnelName(loan.Personnel))},
		{"Jumlah", fmt.Sprintf("<strong style=\"color:#0f172a\">Rp %s</strong>", formatRupiah(amount))},
		{"Tenor", fmt.Sprintf("%d bulan", tenor)},
	}
	if loan.MonthlyInstallment > 0 {
		rows = append(rows, [2]string{"Cicilan / Bulan", fmt.Sprintf("Rp %s", formatRupiah(loan.MonthlyInstallment))})
	}
	if loan.TotalRepayment > 0 {
		rows = append(rows, [2]string{"Total Pengembalian", fmt.Sprintf("Rp %s", formatRupiah(loan.TotalRepayment))})
	}
	if strings.TrimSpace(loan.Purpose) != "" {
		rows = append(rows, [2]string{"Tujuan", htmlEscape(loan.Purpose)})
	}
	return kvCard(rows)
}

func personnelName(p *model.Personnel) string {
	if p == nil || p.FullName == "" {
		return "Pemohon"
	}
	return p.FullName
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}
