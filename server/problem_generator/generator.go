// server/problem_generator/generator.go
package problemgenerator

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"text/template"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Generator struct {
	Repo TemplateRepo
	rng  *rand.Rand
}

func NewGenerator(repo TemplateRepo) *Generator {
	return &Generator{
		Repo: repo,
		rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateRequest defines the input for generating a problem set.
// It includes the user's name, the type of operation, number of problems,
func (g *Generator) GenerateProblemSet(ctx context.Context, req GenerateRequest) (ProblemSet, error) {
	// 1. validation
	if req.NumProblems < 1 || req.NumProblems > 50 {
		return ProblemSet{}, fmt.Errorf("num_problems out of range")
	}
	// 2. fetch candidate templates
	templates, err := g.Repo.ListByOperation(ctx, req.Operation)
	if err != nil {
		return ProblemSet{}, err
	}
	if len(templates) == 0 {
		return ProblemSet{}, fmt.Errorf("no templates for criteria")
	}

	problems := make([]Problem, req.NumProblems)
	for i := 0; i < req.NumProblems; i++ {
		tmpl := templates[g.rng.Intn(len(templates))]
		p, err := g.instantiate(tmpl.Template, req, i+1)
		if err != nil {
			return ProblemSet{}, err
		}
		problems[i] = p
	}
	return ProblemSet{Problems: problems, MetaInfo: req}, nil
}

// helper --------------------------------------------------------------

type tplData struct {
	Name, Pronoun, Noun1, Verb1 string
	Num1, Num2                  int
	PronounLower                string
}

func (g *Generator) instantiate(tpl string, req GenerateRequest, idx int) (Problem, error) {
	num1 := g.rng.Intn(req.MaxNumber) + 1
	num2 := g.rng.Intn(req.MaxNumber) + 1

	if req.Operation == "subtraction" {
		// ensure num1 is always less than num2 for subtraction problems
		if num1 > num2 {
			num1, num2 = num2, num1
		}
	}
	if req.Operation == "division" {
		// ensure num1 is always less than num2 for division problems
		if num1 > num2 {
			num1, num2 = num2, num1
		}
	}

	// Select random noun and verb from user's preferences
	noun := req.LikesNouns[g.rng.Intn(len(req.LikesNouns))]
	verb := req.LikesVerbs[g.rng.Intn(len(req.LikesVerbs))]
	pron := "they"
	title := cases.Title(language.English).String(pron)
	data := tplData{
		Name: req.Name, Pronoun: title, PronounLower: strings.ToLower(pron),
		Noun1: noun, Verb1: verb, Num1: num1, Num2: num2,
	}

	// Execute Go template
	t, err := template.New("p").Parse(tpl)
	if err != nil {
		return Problem{}, err
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return Problem{}, err
	}

	answer := computeAnswer(req.Operation, num1, num2)

	return Problem{
		Index: idx, Text: sb.String(),
		Numbers:   []int{num1, num2},
		Operation: req.Operation,
		Answer:    answer,
	}, nil
}

func computeAnswer(op string, a, b int) string {
	switch op {
	case "addition":
		return fmt.Sprintf("%d + %d = %d", a, b, a+b)
	case "subtraction":
		return fmt.Sprintf("%d - %d = %d", b, a, b-a)
	case "multiplication":
		return fmt.Sprintf("%d * %d = %d", a, b, a*b)
	case "division":
		return fmt.Sprintf("%d / %d = %d", a, b, a/b)
	}
	return fmt.Sprintf("unknown operation %s", op)
}
