package problemgenerator

import pb "github.com/qjs/mathgen_gemma/server/proto"

// Agent parses a plain-text response from the LLM into a ProblemSet.
type Agent interface {
	Parse(llmOut string, req *pb.GenerateRequest) (*ProblemSet, error)
}
