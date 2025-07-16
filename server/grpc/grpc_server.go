// server/grpc/grpc_server.go
package grpcSrv

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	pdfg "github.com/qjs/mathgen_gemma/server/pdf_generator"
	pg "github.com/qjs/mathgen_gemma/server/problem_generator"
	"github.com/qjs/mathgen_gemma/server/prompts"
	pb "github.com/qjs/mathgen_gemma/server/proto"

	api "github.com/ollama/ollama/api"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the Generator gRPC service using the Ollama Go SDK.
type Server struct {
	pb.UnimplementedGeneratorServer
	pdfgen *pdfg.PDFGenerator
	client *api.Client
	model  string
	agent  pg.Agent
}

func NewServer(pdfgen *pdfg.PDFGenerator, ollamaBaseURL, model string, agent pg.Agent) *Server {
	base, err := url.Parse(ollamaBaseURL)
	if err != nil {
		log.Fatalf("invalid Ollama URL: %v", err)
	}

	httpClient := &http.Client{
		Timeout: 120 * time.Second, // whatever is sensible for your env
	}

	client := api.NewClient(base, httpClient)

	return &Server{
		pdfgen: pdfgen,
		client: client,
		model:  model,
		agent:  agent,
	}
}

// GenerateProblemSet queries Ollama for a JSON-formatted problem set and converts it to protobuf.
func (s *Server) GenerateProblemSet(ctx context.Context, req *pb.GenerateRequest) (*pb.ProblemSet, error) {
	//------------------------------------------------------------------
	// 1. Build the prompt with our Kid-Friendly style
	//------------------------------------------------------------------
	pbldr := prompts.Builder{
		Style: prompts.StyleKidFriendly,
		Model: s.model,
	}
	prompt, err := pbldr.Build(req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "prompt build: %v", err)
	}

	//------------------------------------------------------------------
	// 2. Ask Ollama
	//------------------------------------------------------------------
	stream := false
	// format := json.RawMessage(`"text"`)
	gReq := &api.GenerateRequest{
		Model:  s.model,
		Prompt: prompt,
		Stream: &stream,
		// Format: format,
	}

	var responseText string
	if err := s.client.Generate(ctx, gReq, func(gr api.GenerateResponse) error {
		responseText += gr.Response
		return nil
	}); err != nil {
		fmt.Printf("ollama generate: %v\n", err)
		return nil, status.Errorf(codes.Internal, "ollama generate: %v", err)
	}

	ps, err := s.agent.Parse(responseText, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "parse LLM output: %v", err)
	}
	return convertFromInternal(ps), nil
}

// GenerateProblemSetPDF renders PDF from a protobuf ProblemSet.
func (s *Server) GenerateProblemSetPDF(ctx context.Context, psReq *pb.ProblemSet) (*pb.PDFResponse, error) {
	tmp, err := os.CreateTemp("", "problem_set_*.pdf")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "tmp file: %v", err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	if err := s.pdfgen.GeneratePDF(ctx, *convertToInternal(psReq), tmp.Name()); err != nil {
		return nil, status.Errorf(codes.Internal, "pdf gen: %v", err)
	}
	data, err := os.ReadFile(tmp.Name())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read pdf: %v", err)
	}
	return &pb.PDFResponse{Pdf: data, Filename: "problem_set.pdf"}, nil
}

// convertToInternal converts a protobuf ProblemSet to the internal pg.ProblemSet used by the PDF generator.
func convertToInternal(pbps *pb.ProblemSet) *pg.ProblemSet {
	problems := make([]pg.Problem, len(pbps.Problems))
	for i, p := range pbps.Problems {
		nums := make([]int, len(p.Numbers))
		for j, n := range p.Numbers {
			nums[j] = int(n)
		}
		problems[i] = pg.Problem{
			Index:     int(p.Index),
			Text:      p.Text,
			Numbers:   nums,
			Operation: p.Operation,
			Answer:    p.Answer,
		}
	}
	meta := pg.GenerateRequest{
		Name:        pbps.Meta.Name,
		Operation:   pbps.Meta.Operation,
		NumProblems: int(pbps.Meta.NumProblems),
		MaxNumber:   int(pbps.Meta.MaxNumber),
		LikesNouns:  pbps.Meta.LikesNouns,
		LikesVerbs:  pbps.Meta.LikesVerbs,
	}
	return &pg.ProblemSet{Problems: problems, MetaInfo: meta}
}

// convertToInternal converts a protobuf ProblemSet to the internal pg.ProblemSet used by the PDF generator.
func convertFromInternal(pg *pg.ProblemSet) *pb.ProblemSet {
	problems := make([]*pb.Problem, len(pg.Problems))
	for i, p := range pg.Problems {
		nums := make([]int32, len(p.Numbers))
		for j, n := range p.Numbers {
			nums[j] = int32(n)
		}
		problems[i] = &pb.Problem{
			Index:     int32(p.Index),
			Text:      p.Text,
			Numbers:   nums,
			Operation: p.Operation,
			Answer:    p.Answer,
		}
	}
	meta := &pb.GenerateRequest{
		Name:        pg.MetaInfo.Name,
		Operation:   pg.MetaInfo.Operation,
		NumProblems: int32(pg.MetaInfo.NumProblems),
		MaxNumber:   int32(pg.MetaInfo.MaxNumber),
		LikesNouns:  pg.MetaInfo.LikesNouns,
		LikesVerbs:  pg.MetaInfo.LikesVerbs,
	}
	return &pb.ProblemSet{Problems: problems, Meta: meta}
}
