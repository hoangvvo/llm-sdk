package llmagent

import "strings"

type InstructionParam[C any] struct {
	String *string
	Func   func(contextVal C) string
}

// Helper function to build the system prompt from instructions
func getPrompt[C any](instructions []InstructionParam[C], contextVal C) string {
	prompts := make([]string, 0, len(instructions))
	for _, param := range instructions {
		if param.String != nil {
			prompts = append(prompts, *param.String)
		} else if param.Func != nil {
			prompts = append(prompts, param.Func(contextVal))
		}
	}

	return strings.Join(prompts, "\n")
}
