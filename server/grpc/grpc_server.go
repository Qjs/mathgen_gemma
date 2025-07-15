// server/grpc/grpc_server.go
package grpcSrv

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	pdfg "github.com/qjs/mathgen_gemma/server/pdf_generator"
	pg "github.com/qjs/mathgen_gemma/server/problem_generator"
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
}

// NewServer creates a new gRPC Server with the given PDF generator, Ollama base URL, and model name.
func NewServer(pdfgen *pdfg.PDFGenerator, ollamaBaseURL, model string) *Server {
	// ollamaBaseURL like "http://localhost:11434"
	base, _ := url.Parse(ollamaBaseURL)
	client := api.NewClient(base, nil)
	return &Server{pdfgen: pdfgen, client: client, model: model}
}

// GenerateProblemSet queries Ollama for a JSON-formatted problem set and converts it to protobuf.
func (s *Server) GenerateProblemSet(ctx context.Context, req *pb.GenerateRequest) (*pb.ProblemSet, error) {
	// Build prompt
	prompt := fmt.Sprintf(
		"Generate %d %s problems with numbers up to %d in pure JSON (no markdown). The JSON schema: {\"problems\":[{\"index\":int,\"text\":string,\"numbers\":[int,int],\"operation\":string,\"answer\":string}],\"meta\":{\"name\":string,\"operation\":string,\"num_problems\":int,\"max_number\":int,\"likes_nouns\":[string],\"likes_verbs\":[string]}}. Use name=\"%s\", operation=\"%s\".",
		req.NumProblems, strings.ToLower(req.Operation), req.MaxNumber,
		req.Name, req.Operation,
	)

	// Prepare GenerateRequest (non‑stream, JSON format)
	stream := false
	format := json.RawMessage(`"json"`)
	gReq := &api.GenerateRequest{
		Model:  s.model,
		Prompt: prompt,
		Stream: &stream,
		Format: format,
	}

	var responseText string
	// api.Client.Generate streams via callback even when Stream==false; we accumulate.
	err := s.client.Generate(ctx, gReq, func(gr api.GenerateResponse) error {
		responseText += gr.Response
		return nil
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ollama generate error: %v", err)
	}

	// Unmarshal JSON into protobuf ProblemSet
	var ps pb.ProblemSet
	if err := json.Unmarshal([]byte(responseText), &ps); err != nil {
		return nil, status.Errorf(codes.Internal, "JSON parse error: %v LLM output: %s", err, responseText)
	}
	return &ps, nil
}

// GenerateProblemSetPDF renders PDF from a protobuf ProblemSet.
func (s *Server) GenerateProblemSetPDF(ctx context.Context, psReq *pb.ProblemSet) (*pb.PDFResponse, error) {
	tmp, err := ioutil.TempFile("", "problem_set_*.pdf")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "tmp file: %v", err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	if err := s.pdfgen.GeneratePDF(ctx, *convertToInternal(psReq), tmp.Name()); err != nil {
		return nil, status.Errorf(codes.Internal, "pdf gen: %v", err)
	}
	data, err := ioutil.ReadFile(tmp.Name())
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
