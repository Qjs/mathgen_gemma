package problemgenerator

import "context"

type GenerateRequest struct {
	Name        string   `json:"name"`
	Gender      string   `json:"gender"`
	Operation   string   `json:"operation"` // add | sub | mul | div
	NumProblems int      `json:"num_problems"`
	GradeLevel  string   `json:"grade_level"`
	LikesNouns  []string `json:"likes_nouns"`
	LikesVerbs  []string `json:"likes_verbs"`
}

type Problem struct {
	Index     int    `json:"index"`
	Theme     string `json:"theme"`
	Text      string `json:"text"`
	Numbers   []int  `json:"numbers"`
	Operation string `json:"operation"`
	Answer    string `json:"answer"`
}

type ProblemSet struct {
	Problems []Problem       `json:"problems"`
	MetaInfo GenerateRequest `json:"MetaInfo"`
}

/* ---- repository abstraction ---- */

type Template struct {
	ID        int
	Operation string
	Template  string
}

type TemplateRepo interface {
	ListByOperation(ctx context.Context, op string) ([]Template, error)
}
