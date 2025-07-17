package prompts

import (
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
	StyleProblemset
)

// Builder holds configuration for generating prompts.
// You can stash model‑specific tweaks here if needed.
type Builder struct {
	Style Style
	Model string // optional: model name for conditional phrasing
}

type Prompt struct {
	System string
	User   string
}

// Build returns a prompt string based on the GenerateRequest and Style.
func (b Builder) Build(req *pb.GenerateRequest) (Prompt, error) {
	var prompt Prompt
	prompt.System = "You are a creative math problem generator. Your task is to create word problems based on the user's preferences.The problems should be tailored to a student named and focus on the requested math operations (Addition/Subtraction/Multiplication/Division).The problems should use numbers up to a max number value"
	switch b.Style {
	case StyleCompact:
		prompt.User = fmt.Sprintf(
			"Generate %d %s problems with numbers up to %d in pure JSON (no markdown).",
			req.NumProblems, strings.ToLower(req.Operation), req.MaxNumber,
		)

	case StyleProblemset:
		// Assemble topics from LikesNouns + LikesVerbs for "Preferred Topics" line.
		topics := append([]string{}, req.LikesNouns...)
		topics = append(topics, req.LikesVerbs...)
		topicsLine := strings.Join(topics, ", ")

		prompt.User = fmt.Sprintf(`
Here's the user's information:
- Name: %s
- Preferred Topics: %s
- Math Operation: %s
- Number of Problems: %d

Please generate %d unique word %s problems that incorporate elements from the user's interests and are solvable using the specified math operation. The problems should be written in clear, engaging language suitable for a Kindergarden student (assume a general elementary/middle school level). 

 **Example Problem Structure (Please aim for similar complexity and style):** **Scenario:** [briefly describe a scenario related to the user's interests] **Problem:** [state the math problem clearly] **"Problem 1","Dinosaur 🦖","Imagine Amelia is exploring a land filled with dino-sauruses! She sees 12 Stegosauruses and 9 Brachiosauruses. How many dinosaurs does Amelia see in all?",”addition”,9,12** **"Problem 2","Space 🚀","Amelia is counting stars in the night sky. She spots 17 blue stars and 6 yellow stars. What is the total number of stars Amelia counts?",”addition”,17,6** **"Problem 3","Unicorn 🦄","Princess Amelia has 11 sparkling unicorn charms and 7 rainbow unicorn stickers. How many unicorn goodies does she have altogether?",”addition”,11,7** **"Problem 4","Volcano 🌋","At the volcano, there are 15 red rocks and 8 black rocks. How many rocks are there in total around the volcano?",”addition”,15,8** **Remember to:**
*   Vary the scenarios and the specific numbers used in the problems.
    
*   Ensure the problems are grammatically correct and easy to understand.
    
*   Clearly state the question being asked.
    
*   Incorporate the user's interests naturally within the problem context.
    
*   Maintain a positive and engaging tone.
    
*   Do not provide a code example just the question.
    
*   List questions in CSV form
    
*   Always start the CSV with a fixed header: "Index","theme","text","operation","num1","num2"
    
*   Add an emoji of the topic of the question next to the interest
    
*   the CSV row should have “Problem Number”, “theme”, “problem text'“, “operation”, num1,num2,
    
*   If the operation is Subtraction or Division ensure that num1, num2 are in the order of the operation (avoid illogical operations based on the problem text
*   Do not generate csv markdown blocks, only the contents of the csv
     `,
			req.Name, topicsLine, req.Operation, req.NumProblems,
			req.NumProblems, strings.ToLower(req.Operation),
		)

	case StyleVerbose:
		fallthrough // default falls back to verbose

	default:
		prompt.User = fmt.Sprintf(
			"You are an expert math teacher. Create %d engaging %s word problems using numbers up to %d. Incorporate the following nouns %v and verbs %v in the story. Provide the output strictly as JSON with fields: index, text, numbers, operation, answer, and a meta object containing the original request parameters. Do NOT embed markdown.",
			req.NumProblems, req.Operation, req.MaxNumber, req.LikesNouns, req.LikesVerbs,
		)
	}
	return prompt, nil
}

// NewBuilder creates a new prompt builder with the specified style.
