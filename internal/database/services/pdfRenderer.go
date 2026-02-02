package services

import (
	"bytes"
	"fmt"
	"io"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/line"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/orientation"
	"github.com/johnfercher/maroto/v2/pkg/props"
)

// PDFRenderer handles the generation of PDF documents using Maroto v2.
type PDFRenderer struct{}

// NewPDFRenderer creates a new instance of the PDF renderer.
func NewPDFRenderer() *PDFRenderer {
	return &PDFRenderer{}
}

// ExcuseData represents the information required to populate an excuse letter template.
type ExcuseData struct {
	StudentName    string
	StudentID      int
	DateRange      string
	Reason         string
	City           string
	SubmissionDate string
	ReferenceID    string
}

// RenderExcuse performs the Maroto v2 PDF generation with improved formatting and fixed struct fields.
func (r *PDFRenderer) RenderExcuse(data ExcuseData) (io.Reader, error) {
	m := maroto.New()

	m.AddRows(
		// --- SENDER INFO ---
		row.New(12).Add(
			col.New(12).Add(
				text.New(data.StudentName, props.Text{Size: 12, Style: fontstyle.Bold, Align: align.Left}),
			),
		),
		row.New(10).Add(
			col.New(12).Add(
				text.New(fmt.Sprintf("Student ID: %d", data.StudentID), props.Text{Size: 9, Align: align.Left}),
			),
		),

		// Spacing before Date
		row.New(20),

		// --- DATE LINE ---
		row.New(15).Add(
			col.New(12).Add(
				text.New(fmt.Sprintf("%s, den %s", data.City, data.SubmissionDate), props.Text{Size: 10, Align: align.Right}),
			),
		),

		row.New(25),

		// --- SUBJECT ---
		row.New(14).Add(
			col.New(12).Add(
				text.New("Entschuldigung für das Fernbleiben vom Unterricht", props.Text{Size: 14, Style: fontstyle.Bold}),
			),
		),
		row.New(5).Add(
			col.New(12).Add(
				line.New(props.Line{
					Thickness:     0.5,
					Orientation:   orientation.Horizontal,
					SizePercent:   100,
					OffsetPercent: 0,
				}),
			),
		),

		row.New(20),

		// --- SALUTATION ---
		row.New(15).Add(
			col.New(12).Add(
				text.New("Sehr geehrte Damen und Herren,", props.Text{Size: 11}),
			),
		),

		// --- MAIN BODY ---
		row.New(12).Add(
			col.New(12).Add(
				text.New(fmt.Sprintf("hiermit möchte ich mein Fernbleiben vom Unterricht im Zeitraum vom %s entschuldigen.", data.DateRange), props.Text{Size: 11}),
			),
		),

		row.New(15),

		// --- REASON ---
		row.New(10).Add(
			col.New(12).Add(
				text.New("Der Grund für meine Abwesenheit war:", props.Text{Size: 11}),
			),
		),
		row.New(12).Add(
			col.New(12).Add(
				text.New(fmt.Sprintf("\"%s\"", data.Reason), props.Text{Size: 11, Style: fontstyle.Italic}),
			),
		),

		row.New(15),

		// --- CLOSING REQUEST ---
		row.New(12).Add(
			col.New(12).Add(
				text.New("Ich bitte Sie, mein Fehlen als entschuldigt zu markieren.", props.Text{Size: 11}),
			),
		),

		row.New(20),

		// --- SIGNATURE BLOCK ---
		row.New(12).Add(
			col.New(12).Add(
				text.New("Mit freundlichen Grüßen,", props.Text{Size: 11}),
			),
		),

		// Large space for physical signature
		row.New(35),

		// Signature Line and Name (using specific line props)
		row.New(5).Add(
			col.New(12).Add(
				line.New(props.Line{
					Thickness:     0.5,
					Orientation:   orientation.Horizontal,
					SizePercent:   40, // Only span 40% of the width
					OffsetPercent: 0,  // Start from the left
				}),
			),
		),
		row.New(10).Add(
			col.New(12).Add(
				text.New(data.StudentName, props.Text{Size: 11, Style: fontstyle.Bold}),
			),
		),

		// Large spacer to push the metadata footer to the very bottom
		row.New(80),

		// --- FOOTER / METADATA ---
		row.New(5).Add(
			col.New(12).Add(
				line.New(props.Line{
					Thickness:     0.2,
					Orientation:   orientation.Horizontal,
					SizePercent:   100,
					OffsetPercent: 0,
				}),
			),
		),
		row.New(8).Add(
			col.New(12).Add(
				text.New("Dieses Dokument wurde automatisch erstellt von ProximaLectio (WebUntis Discord Bot).", props.Text{Size: 7}),
			),
		),
		row.New(8).Add(
			col.New(12).Add(
				text.New(fmt.Sprintf("Referenz-ID: %s", data.ReferenceID), props.Text{Size: 7}),
			),
		),
	)

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF document: %w", err)
	}

	return bytes.NewReader(doc.GetBytes()), nil
}
