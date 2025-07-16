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
	StyleKidFriendly // 🆕 our custom, Amelia-style prompt
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
	case StyleKidFriendly:
		// Assemble topics from LikesNouns + LikesVerbs for "Preferred Topics" line.
		topics := append([]string{}, req.LikesNouns...)
		topics = append(topics, req.LikesVerbs...)
		topicsLine := strings.Join(topics, ", ")

		return fmt.Sprintf(`
You are a creative math problem generator. Your task is to create %d word problems based on the user's preferences.
The problems should be tailored to a student named %s and focus on %s math operations. The problems should use numbers up to %d.

Here's the user's information:
- Name: %s
- Preferred Topics: %s
- Math Operation: %s
- Number of Problems: %d

Please generate %d unique word %s problems that incorporate elements from the user's interests and are solvable using the specified math operation. The problems should be written in clear, engaging language suitable for a Kindergarden student (assume a general elementary/middle school level). 

Example Problem Structure (Please aim for similar complexity and style): 

Scenario: [briefly describe a scenario related to the user's interests]
Problem: [state the math problem clearly] 

Example Output (Format each problem as a separate paragraph): 

Problem 1:
Imagine [user_name] is exploring a land filled with dinosaurs! There are 15 mighty Tyrannosaurus Rexes and 8 gentle Triceratopses. If each dinosaur has 4 strong legs, how many legs are there in total? 

Problem 2:
[user_name] is an astronaut on a mission to a distant planet. The spaceship needs to travel 24 light-years. If the spaceship travels at a speed of 3 light-years per day, how many days will the journey take? 

... (rest of the examples) ... 

Remember to: 

     Vary the scenarios and the specific numbers used in the problems.
     Ensure the problems are grammatically correct and easy to understand.
     Clearly state the question being asked.
     Incorporate the user's interests naturally within the problem context.
     Maintain a positive and engaging tone.
     Do not provide a code example just the question.
     List questions in CSV form
     Add an emoji of the topic of the question next to the interest
     `,
			req.NumProblems, req.Name, strings.ToLower(req.Operation), req.MaxNumber,
			req.Name, topicsLine, req.Operation, req.NumProblems,
			req.NumProblems, strings.ToLower(req.Operation),
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

// NewBuilder creates a new prompt builder with the specified style.
