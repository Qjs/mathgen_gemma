package webapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	pb "github.com/qjs/mathgen_gemma/server/proto"
)

// WebApp wraps a Gin router plus the gRPC client.
type WebApp struct {
	Router     *gin.Engine
	GRPCClient pb.GeneratorClient
	Server     *http.Server
}

// NewWebApp wires routes + templates and returns an instance.
func NewWebApp(grpcClient pb.GeneratorClient) *WebApp {
	router := gin.Default()
	router.LoadHTMLGlob("server/webapp/template/*")

	app := &WebApp{
		Router:     router,
		GRPCClient: grpcClient,
	}
	app.setupRoutes()
	return app
}

// Run starts the HTTP server (non-blocking).
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

// Shutdown gracefully stops the HTTP server.
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
}

// GET /
func (app *WebApp) indexPage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.tmpl", gin.H{})
}

// POST /generatePDF
func (app *WebApp) generatePDF(c *gin.Context) {
	var req pb.GenerateRequest

	// 1️⃣  Prefer uploaded file …
	if file, err := c.FormFile("queryFile"); err == nil {
		f, err := file.Open()
		if err != nil {
			c.String(http.StatusBadRequest, "reading file: %v", err)
			return
		}
		defer f.Close()
		data, _ := io.ReadAll(f)
		if err := json.Unmarshal(data, &req); err != nil {
			c.String(http.StatusBadRequest, "invalid JSON: %v", err)
			return
		}
	} else {
		// 2️⃣  … or fallback to textarea.
		if err := json.Unmarshal([]byte(c.PostForm("queryText")), &req); err != nil {
			c.String(http.StatusBadRequest, "invalid JSON: %v", err)
			return
		}
	}

	// 3️⃣  Call gRPC → PDF.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	problemResp, err := app.GRPCClient.GenerateProblemSet(ctx, &req)
	if err != nil {
		c.String(http.StatusInternalServerError, "gRPC error: %v", err)
		return
	}
	pdfResp, err := app.GRPCClient.GenerateProblemSetPDF(ctx, problemResp)
	if err != nil {
		c.String(http.StatusInternalServerError, "gRPC error: %v", err)
		return
	}
	// 4️⃣  Stream PDF back.
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, pdfResp.Filename))
	c.Data(http.StatusOK, "application/pdf", pdfResp.Pdf)
}
