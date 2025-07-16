// server/problem_generator/generator.go
package problemgenerator

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	pb "github.com/qjs/mathgen_gemma/server/proto"
)

var (
	reInts        = regexp.MustCompile(`\d+`)
	errEmptyCSV   = errors.New("empty CSV from LLM")
	errBadColumns = errors.New("malformed CSV line (want at least Problem,Text)")
)

// CSVAgent implements Agent by reading "Problem,Text" rows and computing answers.
type CSVAgent struct{}

func NewCSVAgent() *CSVAgent { return &CSVAgent{} }

// ----------------------------- core logic ------------------------------

func (a *CSVAgent) Parse(llmOut string, req *pb.GenerateRequest) (*ProblemSet, error) {
	r := csv.NewReader(strings.NewReader(llmOut))
	r.TrimLeadingSpace = true

	var (
		problems []Problem
		rowNo    int
	)
	for {
		rowNo++
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(rec) < 2 {
			return nil, errBadColumns
		}

		// First col → index (strip optional "Problem " prefix)
		idx, _ := strconv.Atoi(strings.TrimPrefix(strings.ToLower(rec[0]), "problem "))
		if idx == 0 {
			idx = rowNo
		}

		text := rec[1]

		// ---------- pull first two ints & compute answer --------------
		aNum, bNum, answer := extractNumbersAndAnswer(text, strings.ToLower(req.Operation))

		problems = append(problems, Problem{
			Index:     idx,
			Text:      text,
			Numbers:   []int{aNum, bNum},
			Operation: strings.ToLower(req.Operation),
			Answer:    answer,
		})
	}

	if len(problems) == 0 {
		return nil, errEmptyCSV
	}

	meta := GenerateRequest{
		Name:        req.Name,
		Operation:   req.Operation,
		NumProblems: int(req.NumProblems),
		MaxNumber:   int(req.MaxNumber),
		LikesNouns:  req.LikesNouns,
		LikesVerbs:  req.LikesVerbs,
	}
	return &ProblemSet{Problems: problems, MetaInfo: meta}, nil
}

// ----------------------------- helpers --------------------------------

func extractNumbersAndAnswer(text, op string) (int, int, string) {
	ints := reInts.FindAllString(text, -1)
	if len(ints) < 2 {
		return 0, 0, "N/A"
	}
	a, _ := strconv.Atoi(ints[0])
	b, _ := strconv.Atoi(ints[1])
	return a, b, computeAnswer(op, a, b)
}

// computeAnswer is the exact helper you supplied.
func computeAnswer(op string, a, b int) string {
	switch op {
	case "addition":
		return fmt.Sprintf("%d + %d = %d", a, b, a+b)
	case "subtraction":
		return fmt.Sprintf("%d - %d = %d", b, a, b-a)
	case "multiplication":
		return fmt.Sprintf("%d * %d = %d", a, b, a*b)
	case "division":
		if b == 0 {
			return "division by zero"
		}
		return fmt.Sprintf("%d / %d = %d", a, b, a/b)
	default:
		return fmt.Sprintf("unknown operation %s", op)
	}
}
