package pdfgenerator

import (
	"bytes"
	"context"
	"html/template"
	"net/url"
	"os"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	pg "github.com/qjs/mathgen_gemma/server/problem_generator"
)

func GeneratePDF(ctx context.Context, ps pg.ProblemSet, outFile string) error {
	// -- 1. fill template ----------------------------------------------------
	tpl, err := template.ParseFiles("server/pdf_generator/template/problems.html")
	if err != nil {
		return err
	}

	data := map[string]any{
		"Title":       ps.MetaInfo.Name + "'s " + ps.MetaInfo.Operation + " Problem Set",
		"AnswerTitle": ps.MetaInfo.Name + "'s " + ps.MetaInfo.Operation + " Answer Key",
		"Problems":    ps.Problems,
	}

	var htmlBuf bytes.Buffer
	if err := tpl.Execute(&htmlBuf, data); err != nil {
		return err
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		// boolean flag (no value part)
		chromedp.Flag("disable-gpu", true),

		// flag that expects a value: headless=new
		chromedp.Flag("headless", "new"),
	)

	// build allocator/context with the option list
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	var pdfBuf []byte
	if err := chromedp.Run(chromeCtx,
		chromedp.Navigate("data:text/html,"+url.PathEscape(htmlBuf.String())),
		// Wait for fonts/emoji to load
		chromedp.Sleep(500*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				Do(ctx)
			return err
		}),
	); err != nil {
		return err
	}

	// -- 3. save -------------------------------------------------------------
	return os.WriteFile(outFile, pdfBuf, 0o644)
}
