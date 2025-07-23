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
	pg "github.com/qjs/mathgen_gemma/server/problem_generator"
	pb "github.com/qjs/mathgen_gemma/server/proto"
	"github.com/qjs/mathgen_gemma/server/webapp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	outDir   = flag.String("out_dir", "./output", "directory to write JSON + PDF results")
	grpcPort = flag.String("grpc-port", ":50051", "gRPC server port")
	ollama   = flag.String("ollama_url", "http://localhost:11434", "base URL of Ollama API")
	model    = flag.String("model", "gemma3n:e4b", "model name to pass to Ollama")
	webPort  = flag.String("web_port", ":8081", "port for Gin web UI")
)

func main() {
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

	// agent := pg.NewCSVAgent()
	agent := pg.NewJSONAgent()

	svc := grpcSrv.NewServer(*ollama, *model, agent)

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

	webApp := webapp.NewWebApp(client, *outDir)
	go webApp.Run(*webPort)

	<-ctx.Done()
	log.Println("⏹  shutting down …")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := webApp.Shutdown(shutdownCtx); err != nil {
		log.Printf("Web app shutdown error: %v", err)
	}
	grpcServer.GracefulStop() // GracefulStop is blocking until all RPCs finish or timeout
	log.Println("✅ Servers shut down.")
}
