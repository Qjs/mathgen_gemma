// Package webapp hosts a web platform
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
		tempDir:    outputDir,
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
	app.Router.GET("/", app.homePage)
	app.Router.GET("/worksheet", app.formPage)
	app.Router.POST("/generatePDF", app.generatePDF)
	app.Router.POST("/generateInteractive", app.generateInteractive)
	app.Router.GET("/download/:id", app.downloadPDF)
}

// GET /
func (app *WebApp) homePage(c *gin.Context) {
	c.HTML(http.StatusOK, "home", gin.H{
		"Title": "Personalized Math Worksheets",
	})
}

func (app *WebApp) formPage(c *gin.Context) {
	c.HTML(http.StatusOK, "worksheet", gin.H{
		"Title": "Generate a PDF Problem-Set",
	})
}

// POST /generatePDF  (htmx request)
func (app *WebApp) generatePDF(c *gin.Context) {
	req := extractRequestFromForm(c)

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
		fmt.Printf("Failed to generate Problems %v\n", err)
		c.String(http.StatusInternalServerError, "gRPC error: %v", err)
		return
	}
	// 3️⃣  Persist PDF to temp dir
	id := uuid.NewString()
	filename := fmt.Sprintf("%s_%s", id, pdfResp.Filename)
	fullPath := filepath.Join(app.tempDir, filename)
	fmt.Printf("Saving PDF to %s\n", fullPath)
	if err := os.MkdirAll(app.tempDir, 0o755); err != nil {
		fmt.Printf("Failed to create dir\n")

		c.String(http.StatusInternalServerError, "creating output dir: %v", err)
		return
	}
	if err := os.WriteFile(fullPath, pdfResp.Pdf, 0o644); err != nil {
		fmt.Printf("Failed to save pdf\n")

		c.String(http.StatusInternalServerError, "saving PDF: %v", err)
		return
	}

	// 4️⃣  Return an HTML snippet (htmx swaps it into #status)
	c.HTML(http.StatusOK, "snippet_success.tmpl", gin.H{
		"ID":       id,
		"Filename": pdfResp.Filename,
	})
}

// POST generateInteractive
func (app *WebApp) generateInteractive(c *gin.Context) {
	req := extractRequestFromForm(c)

	// 2️⃣  Call gRPC → PDF
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()

	problemResp, err := app.GRPCClient.GenerateProblemSet(ctx, req)
	if err != nil {
		c.String(http.StatusInternalServerError, "gRPC error: %v", err)
		return
	}
	fmt.Printf("Generated %d problems\n", len(problemResp.Problems))

	// Render a full page; not htmx snippet
	c.HTML(http.StatusOK, "interactive", gin.H{
		"Title":     "Interactive Worksheet",
		"Problems":  problemResp.Problems, // slice of {Text, Answer}
		"Student":   req.Name,
		"Operation": req.Operation,
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
	// serve with attachment header – forces “Save as…”
	c.FileAttachment(filePath, filepath.Base(filePath)[37:]) // strips UUID_
}

/*
	Helpers
*/
// extractRequestFromForm

func extractRequestFromForm(c *gin.Context) *pb.GenerateRequest {
	// 1️⃣  Pull values from the HTML form
	name := strings.TrimSpace(c.PostForm("name"))
	gender := strings.TrimSpace(c.PostForm("gender"))
	operation := strings.TrimSpace(c.PostForm("operation"))   // e.g. add, subtract…
	numProblems, _ := strconv.Atoi(c.PostForm("numProblems")) // default to 10
	if numProblems <= 0 {
		numProblems = 10
	}

	gradeLevel := strings.TrimSpace(c.PostForm("gradeLevel"))

	likesNouns := splitCSV(c.PostForm("likesNouns")) // helper below
	likesVerbs := splitCSV(c.PostForm("likesVerbs"))

	fmt.Printf("Generating %d %s problems for %s at a %s level\n", numProblems, operation, name, gradeLevel)

	req := &pb.GenerateRequest{
		Name:        name,
		Gender:      gender,
		Operation:   operation,
		NumProblems: int32(numProblems),
		GradeLevel:  gradeLevel,
		LikesNouns:  likesNouns,
		LikesVerbs:  likesVerbs,
	}
	return req
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
