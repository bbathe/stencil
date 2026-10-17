package pdf

import (
	"fmt"
	"io"
	"strings"
)

// writePDF serializes page content streams into a minimal PDF-1.4 file
// using built-in Helvetica / Helvetica-Bold / Helvetica-Oblique / Courier.
func writePDF(w io.Writer, pages []string) error {
	if len(pages) == 0 {
		pages = []string{""}
	}

	type deferred struct {
		id   int
		body string
	}
	var bodies []deferred
	next := 1
	alloc := func() int {
		id := next
		next++
		return id
	}

	f1, f2, f3, f4 := alloc(), alloc(), alloc(), alloc()
	contentIDs := make([]int, len(pages))
	for i := range pages {
		contentIDs[i] = alloc()
	}
	pageIDs := make([]int, len(pages))
	for i := range pages {
		pageIDs[i] = alloc()
	}
	pagesID := alloc()
	catalogID := alloc()

	add := func(id int, body string) { bodies = append(bodies, deferred{id, body}) }

	add(f1, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	add(f2, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	add(f3, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Oblique /Encoding /WinAnsiEncoding >>")
	add(f4, "<< /Type /Font /Subtype /Type1 /BaseFont /Courier /Encoding /WinAnsiEncoding >>")

	resources := fmt.Sprintf(
		"<< /Font << /F1 %d 0 R /F2 %d 0 R /F3 %d 0 R /F4 %d 0 R >> >>",
		f1, f2, f3, f4,
	)
	for i, content := range pages {
		add(contentIDs[i], fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
	}
	var kids strings.Builder
	kids.WriteByte('[')
	for i, id := range pageIDs {
		if i > 0 {
			kids.WriteByte(' ')
		}
		fmt.Fprintf(&kids, "%d 0 R", id)
		add(id, fmt.Sprintf(
			"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 612 792] /Contents %d 0 R /Resources %s >>",
			pagesID, contentIDs[i], resources,
		))
	}
	kids.WriteByte(']')
	add(pagesID, fmt.Sprintf("<< /Type /Pages /Kids %s /Count %d >>", kids.String(), len(pageIDs)))
	add(catalogID, fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pagesID))

	byID := make([]string, next)
	for _, d := range bodies {
		byID[d.id] = d.body
	}

	var buf strings.Builder
	buf.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")
	offsets := make([]int, next)
	for id := 1; id < next; id++ {
		offsets[id] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", id, byID[id])
	}

	xrefPos := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", next)
	buf.WriteString("0000000000 65535 f \n")
	for id := 1; id < next; id++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[id])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", next, catalogID, xrefPos)

	_, err := io.WriteString(w, buf.String())
	return err
}
