package services

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/line"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/orientation"
	"github.com/johnfercher/maroto/v2/pkg/core"
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
	StudentID      int64
	DateRange      string
	StartTime      string
	EndTime        string
	Reason         string
	City           string
	SubmissionDate string
	ReferenceID    string
	Guardian       string
}

func (r *PDFRenderer) RenderExcuse(data ExcuseData) (io.Reader, error) {
	m := maroto.New()

	m.AddRows(
		row.New(8).Add(
			col.New(12).Add(
				text.New(data.StudentName, props.Text{Size: 11, Style: fontstyle.Bold, Align: align.Left}),
			),
		),
		row.New(6).Add(
			col.New(12).Add(
				text.New(fmt.Sprintf("Student ID: %d", data.StudentID), props.Text{Size: 9, Align: align.Left}),
			),
		),

		row.New(10),

		row.New(8).Add(
			col.New(12).Add(
				text.New(fmt.Sprintf("%s, den %s", data.City, data.SubmissionDate), props.Text{Size: 10, Align: align.Right}),
			),
		),

		row.New(15),

		row.New(10).Add(
			col.New(12).Add(
				text.New("Entschuldigung für das Fernbleiben vom Unterricht", props.Text{Size: 13, Style: fontstyle.Bold}),
			),
		),
		row.New(4).Add(
			col.New(12).Add(
				line.New(props.Line{
					Thickness:     0.5,
					Orientation:   orientation.Horizontal,
					SizePercent:   100,
					OffsetPercent: 0,
				}),
			),
		),

		row.New(10),

		row.New(8).Add(
			col.New(12).Add(
				text.New("Sehr geehrte Damen und Herren,", props.Text{Size: 10}),
			),
		),

		row.New(6),

		row.New(10).Add(
			col.New(12).Add(
				text.New(fmt.Sprintf("hiermit möchte ich mein Fernbleiben vom Unterricht im Zeitraum vom %s (%s - %s Uhr) entschuldigen.", data.DateRange, data.StartTime, data.EndTime), props.Text{Size: 10}),
			),
		),

		row.New(6),

		row.New(6).Add(
			col.New(12).Add(
				text.New("Der Grund für meine Abwesenheit war:", props.Text{Size: 10}),
			),
		),
		row.New(8).Add(
			col.New(12).Add(
				text.New(fmt.Sprintf("\"%s\"", data.Reason), props.Text{Size: 10, Style: fontstyle.Italic}),
			),
		),

		row.New(6),

		row.New(8).Add(
			col.New(12).Add(
				text.New("Ich bitte Sie, mein Fehlen als entschuldigt zu markieren.", props.Text{Size: 10}),
			),
		),

		row.New(10),

		row.New(8).Add(
			col.New(12).Add(
				func() core.Component {
					closing := "Mit freundlichen Grüßen,"
					if data.Guardian != "" {
						closing = fmt.Sprintf("Mit freundlichen Grüßen, %s", data.StudentName)
					}
					return text.New(closing, props.Text{Size: 10})
				}(),
			),
		),

		row.New(20),

		row.New(2).Add(
			col.New(5).Add(
				line.New(props.Line{
					Thickness:     0.3,
					Orientation:   orientation.Horizontal,
					SizePercent:   100,
					OffsetPercent: 0,
				}),
			),
		),
		row.New(8).Add(
			col.New(5).Add(
				func() core.Component {
					name := data.StudentName
					if data.Guardian != "" {
						name = data.Guardian + " (Erziehungsberechtigte/r)"
					}
					return text.New(name, props.Text{Size: 10, Style: fontstyle.Bold})
				}(),
			),
		),

		row.New(30),

		row.New(4).Add(
			col.New(12).Add(
				line.New(props.Line{
					Thickness:     0.2,
					Orientation:   orientation.Horizontal,
					SizePercent:   100,
					OffsetPercent: 0,
				}),
			),
		),
		row.New(6).Add(
			col.New(12).Add(
				text.New("Dieses Dokument wurde automatisch erstellt von ProximaLectio (WebUntis Discord Bot).", props.Text{Size: 6}),
			),
		),
		row.New(6).Add(
			col.New(12).Add(
				text.New(fmt.Sprintf("Referenz-ID: %s", data.ReferenceID), props.Text{Size: 6}),
			),
		),
	)

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF document: %w", err)
	}

	return bytes.NewReader(doc.GetBytes()), nil
}

func formatUntisName(name string) string {
	if strings.Contains(name, ",") {
		parts := strings.Split(name, ",")
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1]) + " " + strings.TrimSpace(parts[0])
		}
	}

	parts := strings.Fields(name)
	if len(parts) == 2 {
		if isLikelySurname(parts[0]) {
			return parts[1] + " " + parts[0]
		}
	}
	return name
}

func isLikelySurname(part string) bool {
	return strings.ToUpper(part) == part && len(part) > 1
}
