package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcSrv "github.com/qjs/mathgen_gemma/server/grpc"
	pdfg "github.com/qjs/mathgen_gemma/server/pdf_generator"
	pb "github.com/qjs/mathgen_gemma/server/proto"
	"github.com/qjs/mathgen_gemma/server/webapp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	//------------------------------------------------------------
	// CLI flags
	//------------------------------------------------------------

	var (
		outDir   = flag.String("out_dir", "./output", "directory to write JSON + PDF results")
		grpcPort = flag.String("grpc-port", ":50051", "gRPC server port")
		ollama   = flag.String("ollama_url", "http://localhost:11434", "base URL of Ollama API")
		model    = flag.String("model", "gemma3n:latest", "model name to pass to Ollama")
		webPort  = flag.String("web_port", ":8080", "port for Gin web UI")
	)
	flag.Parse()

	//------------------------------------------------------------
	// Graceful-shutdown context
	//------------------------------------------------------------
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	//------------------------------------------------------------
	// Output folder
	//------------------------------------------------------------
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", *outDir, err)
	}

	pdfGen := pdfg.NewPDFGenerator(pdfg.Config{
		PageSize:     "Letter",
		MarginsMM:    20,
		FontFamily:   "Helvetica",
		PrimaryColor: [3]int{20, 20, 20},
		Timeout:      30 * time.Second,
	})
	svc := grpcSrv.NewServer(pdfGen, *ollama, *model) // <-- matches new signature

	grpcServer := grpc.NewServer()
	pb.RegisterGeneratorServer(grpcServer, svc)

	// Initialize gRPC Server
	lis, err := net.Listen("tcp", *grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *grpcPort, err)
	}
	// Establish gRPC client connection
	conn, err := grpc.NewClient("localhost"+*grpcPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect GRPC on %s: %v", *grpcPort, err)
	}
	go func() {
		log.Printf("gRPC server listening on %s", *grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC server: %v", err)
		}
	}()

	client := pb.NewGeneratorClient(conn)

	// add flag

	// after the gRPC client is created:
	webApp := webapp.NewWebApp(client)
	go webApp.Run(*webPort)

	// … then, inside the shutdown section:
	<-ctx.Done()
	log.Println("⏹  shutting down …")
	_ = webApp.Shutdown(context.Background())
	grpcServer.GracefulStop()

	//------------------------------------------------------------
	// 4.  Wait for CTRL-C and shut down
	//------------------------------------------------------------
	<-ctx.Done()
	log.Println("⏹  SIGINT/SIGTERM received; stopping server …")
	grpcServer.GracefulStop()
}
