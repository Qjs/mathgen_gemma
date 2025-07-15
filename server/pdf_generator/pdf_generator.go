package pdfgenerator

import (
	"context"
	"fmt"
	"time"

	pg "github.com/qjs/mathgen_gemma/server/problem_generator"

	"codeberg.org/go-pdf/fpdf"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Config struct {
	PageSize     string
	MarginsMM    float64
	FontFamily   string
	PrimaryColor [3]int
	Timeout      time.Duration
}

type PDFGenerator struct {
	cfg Config
}

func NewPDFGenerator(cfg Config) *PDFGenerator {
	return &PDFGenerator{cfg}
}

func (p *PDFGenerator) GeneratePDF(ctx context.Context, ps pg.ProblemSet, outputFilePath string) error {
	if dl, ok := ctx.Deadline(); ok {
		time.AfterFunc(time.Until(dl), func() { /* abort if still running */ })
	}
	pdf := fpdf.New("P", "mm", p.cfg.PageSize, "")
	pdf.SetMargins(p.cfg.MarginsMM, p.cfg.MarginsMM, p.cfg.MarginsMM)

	pdf.SetTitle(fmt.Sprintf("%s's %s Problem Set", ps.MetaInfo.Name, cases.Title(language.English).String(ps.MetaInfo.Operation)), false)
	pdf.AddPage()

	// ---------- title ----------
	pdf.SetFont(p.cfg.FontFamily, "B", 22)
	title := fmt.Sprintf("%s's %s Problem Set", ps.MetaInfo.Name, cases.Title(language.English).String(ps.MetaInfo.Operation))
	pdf.CellFormat(0, 15, title, "", 1, "C", false, 0, "")
	pdf.Ln(10)

	// ---------- problems ----------
	pdf.SetFont(p.cfg.FontFamily, "", 14)
	for _, problem := range ps.Problems {
		txt := fmt.Sprintf("%d. %s", problem.Index, problem.Text)
		pdf.MultiCell(0, 8, txt, "", "L", false)
		txtResp := "Answer: _____"
		pdf.MultiCell(0, 8, txtResp, "", "L", false)
	}

	pdf.AddPage()
	answertitle := fmt.Sprintf("%s's %s Problem Set Answer Key", ps.MetaInfo.Name, cases.Title(language.English).String(ps.MetaInfo.Operation))
	pdf.CellFormat(0, 15, answertitle, "", 1, "C", false, 0, "")
	pdf.Ln(10)
	// ---------- answers ----------
	pdf.SetFont(p.cfg.FontFamily, "", 14)
	for _, problem := range ps.Problems {
		txt := fmt.Sprintf("%d. %s", problem.Index, problem.Answer)
		pdf.MultiCell(0, 8, txt, "", "L", false)

	}

	return pdf.OutputFileAndClose(outputFilePath)
}
