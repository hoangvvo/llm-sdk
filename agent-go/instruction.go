package llmagent

import (
	"context"
	"strings"

	"golang.org/x/sync/errgroup"
)

type InstructionParam[C any] struct {
	String *string
	Func   func(ctx context.Context, contextVal C) (string, error)
}

// Helper function to build the system prompt from instructions
func getPrompt[C any](ctx context.Context, instructions []InstructionParam[C], contextVal C) (string, error) {
	prompts := make([]string, len(instructions))
	g, ctx := errgroup.WithContext(ctx)
	for i, param := range instructions {
		i, param := i, param
		g.Go(func() error {
			if param.String != nil {
				prompts[i] = *param.String
			} else if param.Func != nil {
				var err error
				prompts[i], err = param.Func(ctx, contextVal)
				if err != nil {
					return err
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return "", err
	}
	return strings.Join(prompts, "\n"), nil
}
