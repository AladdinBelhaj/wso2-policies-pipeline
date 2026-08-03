package report

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PDFWriter generates a PDF/1.4 document with structured styling.
type PDFWriter struct {
	pages       []string
	currentPage int
	currentY    float64
	margin      float64
	pageWidth   float64
	pageHeight  float64
	title       string
	subtitle    string
	generatedAt time.Time
}

type KeyVal struct {
	Key string
	Val string
}

type StatCard struct {
	Label string
	Value string
}

// NewPDFWriter initializes a new PDF writer instance.
func NewPDFWriter(title, subtitle string) *PDFWriter {
	pw := &PDFWriter{
		margin:      40.0,
		pageWidth:   595.28, // A4 width in points
		pageHeight:  841.89, // A4 height in points
		title:       title,
		subtitle:    subtitle,
		generatedAt: time.Now(),
	}
	pw.NewPage()
	return pw
}

func (pw *PDFWriter) NewPage() {
	pw.pages = append(pw.pages, "")
	pw.currentPage = len(pw.pages) - 1
	pw.currentY = pw.pageHeight - pw.margin

	pw.drawPageHeader()
}

func (pw *PDFWriter) addOp(op string) {
	if pw.currentPage >= 0 && pw.currentPage < len(pw.pages) {
		pw.pages[pw.currentPage] += op + "\n"
	}
}

func (pw *PDFWriter) drawPageHeader() {
	x := pw.margin
	y := pw.pageHeight - 55.0
	w := pw.pageWidth - (2 * pw.margin)
	h := 35.0

	// Navy blue background banner
	pw.addOp("0.12 0.23 0.38 rg") // #1F3A60
	pw.addOp(fmt.Sprintf("%.2f %.2f %.2f %.2f re f", x, y, w, h))

	// White Title
	pw.addOp("BT /F2 12 Tf 1 1 1 rg")
	pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s) Tj ET", x+12.0, y+20.0, pdfEscape(pw.title)))

	// Subtitle
	if pw.subtitle != "" {
		pw.addOp("BT /F1 9 Tf 0.85 0.9 0.95 rg")
		pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s) Tj ET", x+12.0, y+7.0, pdfEscape(pw.subtitle)))
	}

	pw.currentY = y - 15.0
}

func (pw *PDFWriter) EnsureSpace(needed float64) {
	if pw.currentY-needed < pw.margin+35.0 {
		pw.NewPage()
	}
}

func (pw *PDFWriter) AddSectionHeader(title string) {
	pw.EnsureSpace(38.0)
	// Some callers (e.g. AddTextLine) leave the cursor sitting exactly at the
	// previous line's baseline with no trailing buffer of their own. Section
	// headers can't assume any particular amount of space was left behind,
	// so they enforce their own generous gap unconditionally.
	pw.currentY -= 16.0

	x := pw.margin
	y := pw.currentY
	w := pw.pageWidth - (2 * pw.margin)

	// Bold section title
	pw.addOp("BT /F2 11 Tf 0.12 0.23 0.38 rg")
	pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s) Tj ET", x, y, pdfEscape(title)))

	// Underline rule
	pw.addOp("0.12 0.23 0.38 RG 1 w")
	pw.addOp(fmt.Sprintf("%.2f %.2f m %.2f %.2f l S", x, y-3.0, x+w, y-3.0))

	pw.currentY -= 16.0
}

func (pw *PDFWriter) AddMetadataBox(items []KeyVal, statusBadge string) {
	rowHeight := 14.0
	// Single column: one field per row, so long values (e.g. Base URL) get
	// the full box width instead of being squeezed into a half-width column.
	boxHeight := float64(len(items))*rowHeight + 14.0

	pw.EnsureSpace(boxHeight + 8.0)

	x := pw.margin
	w := pw.pageWidth - (2 * pw.margin)
	y := pw.currentY - boxHeight

	// Light blue/gray background box
	pw.addOp("0.95 0.97 0.99 rg 0.8 0.85 0.9 RG 1 w")
	pw.addOp(fmt.Sprintf("%.2f %.2f %.2f %.2f re b", x, y, w, boxHeight))

	// Status Badge
	if statusBadge != "" {
		bgR, bgG, bgB := 0.1, 0.55, 0.2 // green
		if strings.Contains(statusBadge, "FAIL") || strings.Contains(statusBadge, "CANCEL") {
			bgR, bgG, bgB = 0.8, 0.2, 0.2 // red
		} else if strings.Contains(statusBadge, "DRY") || strings.Contains(statusBadge, "PREVIEW") {
			bgR, bgG, bgB = 0.85, 0.5, 0.1 // orange
		}

		badgeW := 95.0
		badgeH := 16.0
		badgeX := x + w - badgeW - 10.0
		badgeY := y + boxHeight - badgeH - 6.0

		pw.addOp(fmt.Sprintf("%.2f %.2f %.2f rg", bgR, bgG, bgB))
		pw.addOp(fmt.Sprintf("%.2f %.2f %.2f %.2f re f", badgeX, badgeY, badgeW, badgeH))

		pw.addOp("BT /F2 8.5 Tf 1 1 1 rg")
		pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s) Tj ET", badgeX+6.0, badgeY+4.0, pdfEscape(statusBadge)))
	}

	// Size the label column to the longest key actually present, instead of a
	// fixed offset, so labels like "API Target Filter:" never crowd the value.
	maxKeyLen := 0
	for _, item := range items {
		if l := len(item.Key); l > maxKeyLen {
			maxKeyLen = l
		}
	}
	labelWidth := float64(maxKeyLen+1)*8.5*0.6 + 12.0 // +1 for ":", 0.6 ~ Helvetica-Bold avg width
	if labelWidth < 90.0 {
		labelWidth = 90.0
	}

	valueX := x + 10.0 + labelWidth
	valueMaxWidth := (x + w - 10.0) - valueX
	if valueMaxWidth < 50.0 {
		valueMaxWidth = 50.0
	}

	currY := y + boxHeight - 14.0
	for _, item := range items {
		pw.addOp("BT /F2 8.5 Tf 0.2 0.25 0.35 rg")
		pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s:) Tj ET", x+10.0, currY, pdfEscape(item.Key)))
		pw.addOp("BT /F1 8.5 Tf 0.1 0.1 0.1 rg")
		pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s) Tj ET", valueX, currY, pdfEscape(truncateText(item.Val, valueMaxWidth, 8.5))))

		currY -= rowHeight
	}

	pw.currentY = y - 10.0
}

func (pw *PDFWriter) AddStatCards(cards []StatCard) {
	if len(cards) == 0 {
		return
	}
	pw.EnsureSpace(40.0)

	totalW := pw.pageWidth - (2 * pw.margin)
	cardW := (totalW - float64(len(cards)-1)*8.0) / float64(len(cards))
	cardH := 32.0
	y := pw.currentY - cardH

	for i, card := range cards {
		x := pw.margin + float64(i)*(cardW+8.0)

		pw.addOp("0.96 0.97 0.98 rg 0.85 0.88 0.92 RG 1 w")
		pw.addOp(fmt.Sprintf("%.2f %.2f %.2f %.2f re b", x, y, cardW, cardH))

		pw.addOp("BT /F2 12 Tf 0.12 0.23 0.38 rg")
		pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s) Tj ET", x+8.0, y+16.0, pdfEscape(card.Value)))

		pw.addOp("BT /F1 7.5 Tf 0.4 0.45 0.5 rg")
		pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s) Tj ET", x+8.0, y+5.0, pdfEscape(card.Label)))
	}

	pw.currentY = y - 10.0
}

func (pw *PDFWriter) AddEntityHeader(kind, name, id, status string) {
	// Two-line box: name + status share the top line; the ID gets its own
	// full-width line below so a full UUID is shown instead of being
	// squeezed into a narrow column next to the name and status badge.
	row1H := 14.0
	row2H := 12.0
	boxH := row1H + row2H

	pw.EnsureSpace(boxH + 6.0)

	x := pw.margin
	w := pw.pageWidth - (2 * pw.margin)
	y := pw.currentY - boxH

	pw.addOp("0.9 0.94 0.97 rg 0.75 0.82 0.9 RG 1 w")
	pw.addOp(fmt.Sprintf("%.2f %.2f %.2f %.2f re b", x, y, w, boxH))

	entityX := x + 8.0
	statusW := 100.0
	statusX := x + w - statusW - 4.0
	nameMaxWidth := statusX - entityX - 8.0

	row1Y := y + row2H + 4.0 // baseline for the top (name/status) line
	row2Y := y + 4.0         // baseline for the bottom (ID) line

	pw.addOp("BT /F2 9.5 Tf 0.12 0.23 0.38 rg")
	nameLabel := truncateText(fmt.Sprintf("%s: %s", kind, name), nameMaxWidth, 9.5)
	pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s) Tj ET", entityX, row1Y, pdfEscape(nameLabel)))

	if status != "" {
		if strings.Contains(status, "FAIL") || strings.Contains(status, "CANCEL") {
			pw.addOp("BT /F2 8 Tf 0.8 0.2 0.2 rg")
		} else {
			pw.addOp("BT /F2 8 Tf 0.1 0.55 0.2 rg")
		}
		statusLabel := truncateText(fmt.Sprintf("[%s]", status), statusW, 8.0)
		pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s) Tj ET", statusX, row1Y+0.5, pdfEscape(statusLabel)))
	}

	// Full ID on its own line, with the whole box width available — a
	// standard 36-char UUID fits comfortably without truncation.
	pw.addOp("BT /F3 7.5 Tf 0.3 0.35 0.4 rg")
	idLabel := truncateTextMono(fmt.Sprintf("ID: %s", id), w-16.0, 7.5)
	pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s) Tj ET", entityX, row2Y, pdfEscape(idLabel)))

	pw.currentY = y - 6.0
}

func (pw *PDFWriter) AddTable(headers []string, rows [][]string, colRatios []float64) {
	if len(headers) == 0 {
		return
	}

	totalW := pw.pageWidth - (2 * pw.margin)
	colWidths := make([]float64, len(headers))
	sumRatio := 0.0
	for _, r := range colRatios {
		sumRatio += r
	}
	if sumRatio == 0 {
		sumRatio = float64(len(headers))
		for i := range colRatios {
			colRatios[i] = 1.0
		}
	}
	for i, r := range colRatios {
		colWidths[i] = totalW * (r / sumRatio)
	}

	rowH := 15.0
	pw.EnsureSpace(rowH*2 + 8.0)

	// Draw Header Row
	y := pw.currentY - rowH
	x := pw.margin

	pw.addOp("0.2 0.3 0.45 rg 0.2 0.3 0.45 RG 1 w")
	pw.addOp(fmt.Sprintf("%.2f %.2f %.2f %.2f re b", x, y, totalW, rowH))

	currX := x
	for i, h := range headers {
		pw.addOp("BT /F2 8 Tf 1 1 1 rg")
		pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s) Tj ET", currX+4.0, y+4.0, pdfEscape(truncateText(h, colWidths[i]-6.0, 8.0))))
		currX += colWidths[i]
	}

	pw.currentY = y

	// Draw Rows
	for rIdx, row := range rows {
		pw.EnsureSpace(rowH)
		y = pw.currentY - rowH
		currX = x

		if rIdx%2 == 1 {
			pw.addOp("0.97 0.98 0.99 rg 0.88 0.9 0.93 RG 0.5 w")
		} else {
			pw.addOp("1 1 1 rg 0.88 0.9 0.93 RG 0.5 w")
		}
		pw.addOp(fmt.Sprintf("%.2f %.2f %.2f %.2f re b", x, y, totalW, rowH))

		for cIdx, cell := range row {
			if cIdx < len(colWidths) {
				pw.addOp("BT /F1 7.5 Tf 0.15 0.15 0.15 rg")
				pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s) Tj ET", currX+4.0, y+4.0, pdfEscape(truncateText(cell, colWidths[cIdx]-6.0, 7.5))))
				currX += colWidths[cIdx]
			}
		}

		pw.currentY = y
	}

	pw.currentY -= 8.0
}

func (pw *PDFWriter) AddTextLine(line string, isBold bool) {
	pw.EnsureSpace(12.0)
	y := pw.currentY - 10.0
	x := pw.margin + 6.0

	if isBold {
		pw.addOp("BT /F2 8 Tf 0.12 0.23 0.38 rg")
	} else {
		pw.addOp("BT /F1 8 Tf 0.2 0.2 0.2 rg")
	}
	pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s) Tj ET", x, y, pdfEscape(line)))

	pw.currentY = y
}

func (pw *PDFWriter) AddBulletList(items []string) {
	for _, item := range items {
		pw.EnsureSpace(11.0)
		y := pw.currentY - 10.0
		x := pw.margin + 12.0

		pw.addOp("BT /F1 7.5 Tf 0.25 0.25 0.25 rg")
		pw.addOp(fmt.Sprintf("%.2f %.2f Td (%s) Tj ET", x, y, pdfEscape(item)))

		pw.currentY = y
	}
	pw.currentY -= 4.0
}

func (pw *PDFWriter) Build() []byte {
	var buf bytes.Buffer
	var offsets []int

	writeString := func(s string) {
		buf.WriteString(s)
	}

	recordObj := func() {
		offsets = append(offsets, buf.Len())
	}

	writeString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	numPages := len(pw.pages)

	// Obj 1: Catalog
	recordObj()
	writeString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Obj 2: Pages
	recordObj()
	var kids []string
	for i := 0; i < numPages; i++ {
		pageObjID := 6 + i*2
		kids = append(kids, fmt.Sprintf("%d 0 R", pageObjID))
	}
	writeString(fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n", strings.Join(kids, " "), numPages))

	// Obj 3: Font Helvetica
	recordObj()
	writeString("3 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>\nendobj\n")

	// Obj 4: Font Helvetica-Bold
	recordObj()
	writeString("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>\nendobj\n")

	// Obj 5: Font Courier
	recordObj()
	writeString("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Courier /Encoding /WinAnsiEncoding >>\nendobj\n")

	// Pages and streams
	for i := 0; i < numPages; i++ {
		pageObjID := 6 + i*2
		streamObjID := pageObjID + 1

		footerStream := fmt.Sprintf(
			"0.8 0.8 0.8 RG 0.5 w %.2f 32.0 m %.2f 32.0 l S BT /F1 7.5 Tf 0.5 0.5 0.5 rg %.2f 20.0 Td (WSO2 pctl Execution Report  |  Generated: %s  |  Page %d of %d) Tj ET\n",
			pw.margin, pw.pageWidth-pw.margin, pw.margin, pw.generatedAt.Format("2006-01-02 15:04:05"), i+1, numPages,
		)

		fullStream := pw.pages[i] + footerStream

		recordObj()
		writeString(fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /Font << /F1 3 0 R /F2 4 0 R /F3 5 0 R >> >> /Contents %d 0 R >>\nendobj\n", pageObjID, pw.pageWidth, pw.pageHeight, streamObjID))

		recordObj()
		writeString(fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj\n", streamObjID, len(fullStream), fullStream))
	}

	startXref := buf.Len()
	totalObjs := 5 + numPages*2

	writeString("xref\n")
	writeString(fmt.Sprintf("0 %d\n", totalObjs+1))
	writeString("0000000000 65535 f \n")
	for _, offset := range offsets {
		writeString(fmt.Sprintf("%010d 00000 n \n", offset))
	}

	writeString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", totalObjs+1, startXref))

	return buf.Bytes()
}

func (pw *PDFWriter) SaveToFile(filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filePath, pw.Build(), 0644)
}

func pdfEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")

	var sb strings.Builder
	for _, r := range s {
		if r >= 32 && r <= 126 {
			sb.WriteRune(r)
		} else if r == '\n' || r == '\t' {
			sb.WriteRune(' ')
		} else {
			sb.WriteRune('?')
		}
	}
	return sb.String()
}

func truncateText(text string, maxWidth float64, fontSize float64) string {
	charWidth := fontSize * 0.52
	maxChars := int(maxWidth / charWidth)
	if maxChars < 3 {
		maxChars = 3
	}
	if len(text) > maxChars {
		return text[:maxChars-2] + ".."
	}
	return text
}

// truncateTextMono is like truncateText but for monospaced fonts (e.g. Courier,
// used for /F3). Courier's advance width is a fixed 0.6em per character, which
// is noticeably wider than the 0.52 heuristic tuned for proportional Helvetica;
// reusing truncateText for Courier text under-truncates and causes overlap
// with neighboring fields.
func truncateTextMono(text string, maxWidth float64, fontSize float64) string {
	charWidth := fontSize * 0.62 // 0.6em nominal + small safety margin
	maxChars := int(maxWidth / charWidth)
	if maxChars < 3 {
		maxChars = 3
	}
	if len(text) > maxChars {
		return text[:maxChars-2] + ".."
	}
	return text
}
