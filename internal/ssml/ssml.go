// Package ssml builds the small amount of SSML the shims need to express a
// speaking-rate override, which Speechify reads from the input text (it has no
// top-level speed field).
package ssml

import (
	"math"
	"strconv"
	"strings"
)

// WrapSpeed wraps plain text in <speak><prosody rate="±N%"> when speed is a
// non-nil, non-1.0 multiplier. Text that already looks like SSML is returned
// untouched so a document root is never double-wrapped. The percentage is
// rounded (not truncated) so e.g. 1.2 yields +20%, not +19%.
func WrapSpeed(text string, speed *float64) string {
	if looksLikeSSML(text) {
		return text
	}
	rate := rate(speed)
	if rate == "" {
		return text
	}
	var b strings.Builder
	b.WriteString(`<speak><prosody rate="`)
	b.WriteString(rate)
	b.WriteString(`">`)
	b.WriteString(escapeXML(text))
	b.WriteString("</prosody></speak>")
	return b.String()
}

func looksLikeSSML(text string) bool {
	t := strings.TrimSpace(text)
	return strings.HasPrefix(t, "<speak") || strings.HasPrefix(t, "<?xml")
}

func rate(speed *float64) string {
	if speed == nil {
		return ""
	}
	s := *speed
	if s <= 0 || (s > 0.99 && s < 1.01) {
		return ""
	}
	pct := int(math.Round((s - 1.0) * 100.0))
	if pct == 0 {
		return ""
	}
	if pct > 0 {
		return "+" + strconv.Itoa(pct) + "%"
	}
	return strconv.Itoa(pct) + "%"
}

// escapeXML escapes the five XML predefined entities so arbitrary user text is
// safe to embed inside an SSML document.
func escapeXML(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	).Replace(s)
}
