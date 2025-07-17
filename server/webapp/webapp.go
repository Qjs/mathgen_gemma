package webapp

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	pb "github.com/qjs/mathgen_gemma/server/proto"
)

type WebApp struct {
	Router     *gin.Engine
	GRPCClient pb.GeneratorClient
	Server     *http.Server
	tempDir    string
}

// NewWebApp wires routes, templates, static assets
func NewWebApp(grpcClient pb.GeneratorClient, outputDir string) *WebApp {
	router := gin.Default()

	// static assets (CSS overrides, favicon, …)
	router.Static("/static", "server/webapp/static")

	// templates (includes layout + partials)
	router.SetFuncMap(template.FuncMap{
		"now": time.Now,
	})
	router.LoadHTMLGlob("server/webapp/template/*")

	app := &WebApp{
		Router:     router,
		GRPCClient: grpcClient,
		tempDir:    outputDir, // or a dedicated work dir
	}
	app.setupRoutes()
	return app
}

// Run starts an HTTP server (non-blocking)
func (app *WebApp) Run(addr string) {
	app.Server = &http.Server{
		Addr:              addr,
		Handler:           app.Router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("🌐  web UI listening on %s", addr)
		if err := app.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("webapp: %v", err)
		}
	}()
}

// Shutdown gracefully stops the HTTP server
func (app *WebApp) Shutdown(ctx context.Context) error {
	if app.Server != nil {
		return app.Server.Shutdown(ctx)
	}
	return nil
}

// ----------------------------------------------------------------------
// Routes
// ----------------------------------------------------------------------

func (app *WebApp) setupRoutes() {
	app.Router.GET("/", app.indexPage)
	app.Router.POST("/generatePDF", app.generatePDF)
	app.Router.GET("/download/:id", app.downloadPDF)
}

// GET /
func (app *WebApp) indexPage(c *gin.Context) {
	c.HTML(http.StatusOK, "index", gin.H{}) // not "index.tmpl"
}

// POST /generatePDF  (htmx request)
func (app *WebApp) generatePDF(c *gin.Context) {
	// 1️⃣  Pull values from the HTML form
	name := strings.TrimSpace(c.PostForm("name"))
	operation := strings.TrimSpace(c.PostForm("operation"))   // e.g. add, subtract…
	numProblems, _ := strconv.Atoi(c.PostForm("numProblems")) // default to 10
	if numProblems <= 0 {
		numProblems = 10
	}

	maxNumber, _ := strconv.Atoi(c.PostForm("maxNumber")) // default to 100
	if maxNumber <= 0 {
		maxNumber = 100
	}

	likesNouns := splitCSV(c.PostForm("likesNouns")) // helper below
	likesVerbs := splitCSV(c.PostForm("likesVerbs"))

	req := &pb.GenerateRequest{
		Name:        name,
		Operation:   operation,
		NumProblems: int32(numProblems),
		MaxNumber:   int32(maxNumber),
		LikesNouns:  likesNouns,
		LikesVerbs:  likesVerbs,
	}
	fmt.Printf("Generating %d %s problems for %s (max number: %d)\n", numProblems, operation, name, maxNumber)
	// 2️⃣  Call gRPC → PDF
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()

	problemResp, err := app.GRPCClient.GenerateProblemSet(ctx, req)
	if err != nil {
		c.String(http.StatusInternalServerError, "gRPC error: %v", err)
		return
	}
	fmt.Printf("Generated %d problems\n", len(problemResp.Problems))
	pdfResp, err := app.GRPCClient.GenerateProblemSetPDF(ctx, problemResp)
	if err != nil {
		c.String(http.StatusInternalServerError, "gRPC error: %v", err)
		return
	}
	// 3️⃣  Persist PDF to temp dir
	id := uuid.NewString()
	filename := fmt.Sprintf("%s_%s", id, pdfResp.Filename)
	fullPath := filepath.Join(app.tempDir, filename)
	fmt.Printf("Saving PDF to %s\n", fullPath)
	if err := os.MkdirAll(app.tempDir, 0o755); err != nil {
		c.String(http.StatusInternalServerError, "creating output dir: %v", err)
		return
	}
	if err := os.WriteFile(fullPath, pdfResp.Pdf, 0o644); err != nil {
		c.String(http.StatusInternalServerError, "saving PDF: %v", err)
		return
	}

	// 4️⃣  Return an HTML snippet (htmx swaps it into #status)
	c.HTML(http.StatusOK, "snippet_success.tmpl", gin.H{
		"ID":       id,
		"Filename": pdfResp.Filename,
	})
}

// GET /download/:id
func (app *WebApp) downloadPDF(c *gin.Context) {
	id := c.Param("id")
	matches, _ := filepath.Glob(filepath.Join(app.tempDir, id+"_*"))
	if len(matches) == 0 {
		c.String(http.StatusNotFound, "file not found")
		return
	}

	filePath := matches[0]
	c.FileAttachment(filePath, filepath.Base(filePath)[37:]) // strip uuid + '_' prefix
}

// splitCSV turns "cat,  dog,fish " → []string{"cat","dog","fish"}
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
