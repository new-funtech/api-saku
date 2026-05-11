package notifier

import (
	"context"
	"fmt"
	"strings"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
)

func (n *Notifier) LeaveDecision(ctx context.Context, leave *model.Leave, status string) {
	if n == nil || leave == nil {
		return
	}
	to := personnelEmail(leave.Personnel)
	if len(to) == 0 {
		return
	}
	approved := strings.EqualFold(status, "approved")
	tone := toneSuccess
	badge := "Disetujui"
	headline := "Pengajuan Cuti / Izin Disetujui"
	intro := "Pengajuan cuti / izin Anda telah <strong style=\"color:#047857\">disetujui</strong> oleh admin perusahaan."
	if !approved {
		tone = toneDanger
		badge = "Ditolak"
		headline = "Pengajuan Cuti / Izin Ditolak"
		intro = "Mohon maaf, pengajuan cuti / izin Anda telah <strong style=\"color:#b91c1c\">ditolak</strong> oleh admin perusahaan."
	}

	totalDays := 0
	if leave.TotalDays != nil {
		totalDays = *leave.TotalDays
	}
	rows := [][2]string{
		{"Jenis", htmlEscape(leaveTypeLabel(leave.Type))},
		{"Tanggal Mulai", formatDateID(leave.StartDate)},
		{"Tanggal Selesai", formatDateID(leave.EndDate)},
		{"Total Hari", fmt.Sprintf("%d hari", totalDays)},
		{"Alasan", htmlEscape(leave.Reason)},
	}
	body := paragraph(intro)
	body += kvCard(rows)
	if note := derefStr(leave.ApproverNotes); note != "" {
		noteTone := toneInfo
		title := "Catatan Persetujuan"
		if !approved {
			noteTone = toneDanger
			title = "Alasan Penolakan"
		}
		body += noticeBox(noteTone, title, htmlEscape(note))
	}
	if approved {
		body += noticeBox(toneSuccess, "Selamat",
			"Anda dapat melihat detail pengajuan kapan saja melalui aplikasi SAKU.")
	} else {
		body += noticeBox(toneInfo, "Butuh Bantuan?",
			"Hubungi admin perusahaan Anda untuk informasi lebih lanjut.")
	}

	html := wrapEmail(headline,
		"Keputusan dari admin perusahaan untuk pengajuan Anda.",
		badge, tone,
		fmt.Sprintf("Pengajuan cuti/izin Anda %s.", strings.ToLower(badge)),
		body)
	subject := fmt.Sprintf("[SAKU] %s — %s", headline, formatDateID(leave.StartDate))
	n.dispatch(ctx, to, subject, html)
}

func (n *Notifier) CorrectionDecision(ctx context.Context, c *model.AttendanceCorrection, status string) {
	if n == nil || c == nil {
		return
	}
	to := personnelEmail(c.Personnel)
	if len(to) == 0 {
		return
	}
	approved := strings.EqualFold(status, "approved")
	tone := toneSuccess
	badge := "Disetujui"
	headline := "Koreksi Absensi Disetujui"
	intro := "Pengajuan koreksi absensi Anda telah <strong style=\"color:#047857\">disetujui</strong> oleh admin perusahaan."
	if !approved {
		tone = toneDanger
		badge = "Ditolak"
		headline = "Koreksi Absensi Ditolak"
		intro = "Mohon maaf, pengajuan koreksi absensi Anda telah <strong style=\"color:#b91c1c\">ditolak</strong> oleh admin perusahaan."
	}

	rows := [][2]string{
		{"Tanggal Koreksi", formatDateID(c.CorrectionDate)},
		{"Jenis Koreksi", htmlEscape(correctionTypeLabel(c.CorrectionType))},
	}
	if c.CheckinTime != nil && *c.CheckinTime != "" {
		rows = append(rows, [2]string{"Jam Masuk", htmlEscape(formatTimeID(*c.CheckinTime))})
	}
	if c.CheckoutTime != nil && *c.CheckoutTime != "" {
		rows = append(rows, [2]string{"Jam Pulang", htmlEscape(formatTimeID(*c.CheckoutTime))})
	}
	rows = append(rows, [2]string{"Alasan", htmlEscape(c.Reason)})

	body := paragraph(intro)
	body += kvCard(rows)
	if note := derefStr(c.ApproverNotes); note != "" {
		noteTone := toneInfo
		title := "Catatan Persetujuan"
		if !approved {
			noteTone = toneDanger
			title = "Alasan Penolakan"
		}
		body += noticeBox(noteTone, title, htmlEscape(note))
	}
	if approved {
		body += noticeBox(toneSuccess, "Selesai",
			"Catatan absensi Anda telah diperbarui sesuai pengajuan.")
	} else {
		body += noticeBox(toneInfo, "Butuh Bantuan?",
			"Hubungi admin perusahaan Anda untuk informasi lebih lanjut atau ajukan kembali sesuai prosedur.")
	}

	html := wrapEmail(headline,
		"Keputusan dari admin perusahaan untuk koreksi absensi Anda.",
		badge, tone,
		fmt.Sprintf("Koreksi absensi Anda %s.", strings.ToLower(badge)),
		body)
	subject := fmt.Sprintf("[SAKU] %s — %s", headline, formatDateID(c.CorrectionDate))
	n.dispatch(ctx, to, subject, html)
}

func leaveTypeLabel(t string) string {
	switch strings.ToLower(t) {
	case "leave":
		return "Cuti"
	case "permission":
		return "Izin"
	case "sick":
		return "Sakit"
	}
	return t
}

func correctionTypeLabel(t string) string {
	switch strings.ToLower(t) {
	case "checkin":
		return "Jam Masuk"
	case "checkout":
		return "Jam Pulang"
	case "both":
		return "Jam Masuk & Pulang"
	}
	return t
}
