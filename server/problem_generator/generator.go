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
// CSV output is as such
// "Index","theme","text","operation","num1","num2"
func (a *CSVAgent) Parse(llmOut string, req *pb.GenerateRequest) (*ProblemSet, error) {
	r := csv.NewReader(strings.NewReader(llmOut))
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1

	var problems []Problem
	recN := 0
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		recN++
		if len(rec) < 6 {
			return nil, fmt.Errorf("line %d: need 6 columns, got %d", recN, len(rec))
		}

		// ---------- pull index and two ints for value & compute answer --------------
		idx, aNum, bNum, answer, err := extractNumbersAndAnswer(rec, strings.ToLower(req.Operation))
		if err != nil {
			return nil, fmt.Errorf("Unable to extract numbers and answer")
		}
		problems = append(problems, Problem{
			Index:     idx,
			Theme:     strings.TrimSpace(rec[1]),
			Text:      strings.TrimSpace(rec[2]),
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

func extractNumbersAndAnswer(text []string, op string) (int, int, int, string, error) {

	idx, _ := strconv.Atoi(text[0])
	a, _ := strconv.Atoi(text[4])
	b, _ := strconv.Atoi(text[5])
	ans, err := computeAnswer(op, a, b)
	if err != nil {
		return 0, 0, 0, "N/A", fmt.Errorf("unable to compute answer")
	}
	return idx, a, b, ans, nil
}

func computeAnswer(op string, num1, num2 int) (string, error) {
	ans := 0
	switch op {
	case "addition", "add", "+":
		ans = num1 + num2
		return fmt.Sprintf("%d", ans), nil
	case "subtraction", "sub", "-":
		ans = num1 - num2
		return fmt.Sprintf("%d", ans), nil

	case "multiplication", "mul", "*", "×":
		ans = num1 * num2
		return fmt.Sprintf("%d", ans), nil

	case "division", "div", "/":
		if num2 == 0 {
			return "0", fmt.Errorf("division by zero")
		}
		ans = num1 / num2
		return fmt.Sprintf("%d", ans), nil

	default:
		return "0", fmt.Errorf("unknown operation %q", op)
	}
}
