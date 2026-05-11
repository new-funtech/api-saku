package notifier

import (
	"fmt"
	"strings"
	"time"
)

type statusTone string

const (
	toneInfo    statusTone = "info"    // blue — submission, info
	toneSuccess statusTone = "success" // green — approved
	toneWarn    statusTone = "warn"    // amber — awaiting confirm
	toneDanger  statusTone = "danger"  // red — rejected, cancelled
)

func toneColors(t statusTone) (gradient, badgeBg, badgeFg, badgeBorder string) {
	switch t {
	case toneSuccess:
		return "linear-gradient(135deg,#10b981 0%,#059669 100%)", "#ecfdf5", "#047857", "#a7f3d0"
	case toneWarn:
		return "linear-gradient(135deg,#f59e0b 0%,#d97706 100%)", "#fffbeb", "#b45309", "#fde68a"
	case toneDanger:
		return "linear-gradient(135deg,#ef4444 0%,#dc2626 100%)", "#fef2f2", "#b91c1c", "#fecaca"
	default:
		return "linear-gradient(135deg,#3b82f6 0%,#2563eb 100%)", "#eff6ff", "#1d4ed8", "#bfdbfe"
	}
}

func wrapEmail(title, subtitle, badgeText string, tone statusTone, preheader, bodyHTML string) string {
	gradient, badgeBg, badgeFg, badgeBorder := toneColors(tone)
	badge := ""
	if badgeText != "" {
		badge = fmt.Sprintf(`<span style="display:inline-block;background-color:%s;color:%s;font-size:11px;font-weight:600;padding:6px 14px;border-radius:999px;border:1px solid %s;white-space:nowrap;letter-spacing:0.3px">%s</span>`, badgeBg, badgeFg, badgeBorder, badgeText)
	}
	if subtitle == "" {
		subtitle = "Notifikasi otomatis dari sistem SAKU"
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="id"><head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1.0">
<meta name="color-scheme" content="light">
<meta name="supported-color-schemes" content="light">
<title>%s</title>
<style>
@media only screen and (max-width:620px){
  .email-wrapper{width:100%%!important}
  .body-pad{padding:24px 20px!important}
  .header-pad{padding:28px 22px!important}
  .stack-grid td{display:block!important;width:100%%!important;border-right:none!important;border-bottom:1px solid #e2e8f0!important}
  .stack-grid td:last-child{border-bottom:none!important}
}
</style>
</head>
<body style="margin:0;padding:0;background-color:#f1f5f9;font-family:'Aptos','Aptos Display',-apple-system,BlinkMacSystemFont,'SF Pro Display','SF Pro Text','Helvetica Neue',Arial,sans-serif;-webkit-font-smoothing:antialiased;-moz-osx-font-smoothing:grayscale;color:#0f172a">
<div style="display:none;max-height:0;overflow:hidden;font-size:1px;line-height:1px;color:#f1f5f9">%s</div>
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f1f5f9">
<tr><td align="center" style="padding:32px 16px">
<table role="presentation" class="email-wrapper" width="600" cellpadding="0" cellspacing="0" style="width:600px;max-width:100%%;background-color:#ffffff;border-radius:18px;overflow:hidden;box-shadow:0 1px 2px rgba(15,23,42,0.04),0 8px 24px rgba(15,23,42,0.06);border:1px solid #e2e8f0">

<!-- Brand strip -->
<tr><td style="background:%s;padding:6px 0"></td></tr>

<!-- Header -->
<tr><td class="header-pad" style="padding:30px 36px 26px;border-bottom:1px solid #f1f5f9">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0"><tr>
<td valign="top">
<p style="margin:0 0 8px;font-size:11px;font-weight:600;color:#94a3b8;letter-spacing:1.4px;text-transform:uppercase">SAKU &middot; Aplikasi Kepegawaian</p>
<h1 style="margin:0 0 6px;font-size:24px;font-weight:700;color:#0f172a;line-height:1.2;letter-spacing:-0.4px">%s</h1>
<p style="margin:0;font-size:13.5px;color:#64748b;line-height:1.5">%s</p>
</td>
<td align="right" valign="top" style="padding-left:12px">%s</td>
</tr></table>
</td></tr>

<!-- Body -->
<tr><td class="body-pad" style="padding:28px 36px 8px">
%s
</td></tr>

<!-- Footer -->
<tr><td style="padding:22px 36px 26px;border-top:1px solid #f1f5f9;background-color:#fafbfc;text-align:center">
<p style="margin:0 0 6px;font-size:12px;color:#94a3b8;line-height:1.6">Email ini dikirim otomatis oleh sistem &mdash; mohon tidak membalas pesan ini.</p>
<p style="margin:0 0 4px;font-size:11.5px;color:#94a3b8">Untuk pertanyaan lebih lanjut, silakan hubungi admin perusahaan Anda atau tim SAKU.</p>
<p style="margin:14px 0 0;font-size:11px;color:#cbd5e1;letter-spacing:0.3px">&copy; %d SAKU &bull; Aplikasi Kepegawaian</p>
</td></tr>

</table>
</td></tr></table>
</body></html>`,
		htmlEscape(title),
		htmlEscape(preheader),
		gradient,
		htmlEscape(title),
		htmlEscape(subtitle),
		badge,
		bodyHTML,
		time.Now().Year(),
	)
}

func kvCard(rows [][2]string) string {
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="border:1px solid #e2e8f0;border-radius:12px;overflow:hidden;margin:0 0 22px">`)
	for i, r := range rows {
		border := "border-bottom:1px solid #f1f5f9"
		if i == len(rows)-1 {
			border = ""
		}
		b.WriteString(fmt.Sprintf(
			`<tr><td style="padding:13px 18px;width:42%%;background-color:#fafbfc;%s"><p style="margin:0;font-size:11px;font-weight:600;color:#64748b;text-transform:uppercase;letter-spacing:0.6px">%s</p></td>`+
				`<td style="padding:13px 18px;%s"><p style="margin:0;font-size:14px;font-weight:500;color:#0f172a">%s</p></td></tr>`,
			border, htmlEscape(r[0]), border, r[1],
		))
	}
	b.WriteString(`</table>`)
	return b.String()
}

func noticeBox(tone statusTone, title, message string) string {
	bg, fg, border := "#eff6ff", "#1e40af", "#bfdbfe"
	switch tone {
	case toneSuccess:
		bg, fg, border = "#ecfdf5", "#065f46", "#a7f3d0"
	case toneWarn:
		bg, fg, border = "#fffbeb", "#854d0e", "#fde68a"
	case toneDanger:
		bg, fg, border = "#fef2f2", "#991b1b", "#fecaca"
	}
	titleHTML := ""
	if title != "" {
		titleHTML = fmt.Sprintf(`<p style="margin:0 0 4px;font-size:13px;font-weight:700;color:%s">%s</p>`, fg, htmlEscape(title))
	}
	return fmt.Sprintf(`<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:%s;border:1px solid %s;border-radius:12px;margin:0 0 22px"><tr><td style="padding:14px 18px">%s<p style="margin:0;font-size:13px;color:%s;line-height:1.6">%s</p></td></tr></table>`,
		bg, border, titleHTML, fg, message,
	)
}

func paragraph(html string) string {
	return fmt.Sprintf(`<p style="margin:0 0 16px;font-size:14.5px;color:#334155;line-height:1.65">%s</p>`, html)
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}
