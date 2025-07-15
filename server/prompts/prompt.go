package prompts

import (
	"encoding/json"
	"fmt"
	"strings"

	pb "github.com/qjs/mathgen_gemma/server/proto"
)

// Style controls the verbosity / format of the prompt sent to the LLM.
// Expand this enum as you experiment with new phrasing strategies.
//
//   - StyleCompact: shortest prompt, just enough to elicit JSON.
//   - StyleSchema: includes an explicit JSON schema example.
//   - StyleVerbose: adds narrative instructions for better creativity.
//
// You can set the desired style when you build your prompt.
// If you add a new style, simply extend the switch in Builder.Build.
// Existing call‑sites only need to swap the enum value.
//
// NOTE: Keeping prompt construction here decouples it from the service
// layer, so you can iterate quickly without touching business logic.

type Style int

const (
	StyleCompact Style = iota
	StyleSchema
	StyleVerbose
)

// Builder holds configuration for generating prompts.
// You can stash model‑specific tweaks here if needed.
type Builder struct {
	Style Style
	Model string // optional: model name for conditional phrasing
}

// Build returns a prompt string based on the GenerateRequest and Style.
func (b Builder) Build(req *pb.GenerateRequest) (string, error) {
	switch b.Style {
	case StyleCompact:
		return fmt.Sprintf(
			"Generate %d %s problems with numbers up to %d in pure JSON (no markdown).",
			req.NumProblems, strings.ToLower(req.Operation), req.MaxNumber,
		), nil

	case StyleSchema:
		// Provide a minimal schema example so the LLM is crystal clear.
		schema, _ := json.Marshal(map[string]any{
			"problems": []any{
				map[string]any{
					"index":     "int",
					"text":      "string",
					"numbers":   []string{"int", "int"},
					"operation": "string",
					"answer":    "string",
				},
			},
			"meta": map[string]any{
				"name":         "string",
				"operation":    "string",
				"num_problems": "int",
				"max_number":   "int",
				"likes_nouns":  []string{"string"},
				"likes_verbs":  []string{"string"},
			},
		})
		return fmt.Sprintf(
			"Return ONLY JSON matching this schema: %s. Fill it with %d %s problems (numbers ≤ %d) for student \"%s\". Use nouns %v and verbs %v.",
			schema,
			req.NumProblems, strings.ToLower(req.Operation), req.MaxNumber, req.Name,
			req.LikesNouns, req.LikesVerbs,
		), nil

	case StyleVerbose:
		fallthrough // default falls back to verbose
	default:
		return fmt.Sprintf(
			"You are an expert math teacher. Create %d engaging %s word problems using numbers up to %d. Incorporate the following nouns %v and verbs %v in the story. Provide the output strictly as JSON with fields: index, text, numbers, operation, answer, and a meta object containing the original request parameters. Do NOT embed markdown.",
			req.NumProblems, req.Operation, req.MaxNumber, req.LikesNouns, req.LikesVerbs,
		), nil
	}
}
