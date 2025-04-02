package main

import (
	"context"
	"encoding/json"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	"github.com/sanity-io/litter"
)

func main() {
	const text = `Grandma's Classic Pancakes
These fluffy pancakes are quick to make and perfect for breakfast!

Ingredients (makes 8 pancakes):

1 cup all-purpose flour
2 tablespoons sugar
1 teaspoon baking powder
1/2 teaspoon baking soda
1/4 teaspoon salt
3/4 cup buttermilk
1/4 cup milk
1 large egg
2 tablespoons unsalted butter, melted

Butter or oil for cooking

Instructions:

In a large bowl, whisk together flour, sugar, baking powder, baking soda, and salt.
In another bowl, mix buttermilk, milk, egg, and melted butter.
Pour the wet ingredients into the dry and stir until just combined (batter will be lumpy—don’t overmix).
Heat a skillet over medium heat and lightly grease with butter or oil.
Pour 1/4 cup batter for each pancake. Cook until bubbles form on the surface, then flip and cook until golden brown.
Serve warm with maple syrup, fresh fruit, or whipped cream.

Prep time: 10 minutes
Cook time: 15 minutes
Total time: 25 minutes

Tags: breakfast, easy, kid-friendly`

	type Output struct {
		Title        string   `json:"title"`
		Description  string   `json:"description"`
		Ingredients  []string `json:"ingredients"`
		Instructions []string `json:"instructions"`
		PrepTime     string   `json:"prep_time"`
		CookTime     string   `json:"cook_time"`
		TotalTime    string   `json:"total_time"`
		Tags         []string `json:"tags"`
	}

	// You can use libraries like invopop/jsonschema to generate JSON schema from Go struct
	// instead of defining it manually like below.
	schema := llmsdk.JSONSchema{
		"title": "Recipe",
		"type":  "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "The name of the recipe.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "A short description of the recipe.",
			},
			"ingredients": map[string]any{
				"type":        "array",
				"description": "List of ingredients required for the recipe.",
				"items": map[string]any{
					"type": "string",
				},
			},
			"instructions": map[string]any{
				"type":        "array",
				"description": "Step-by-step instructions for preparing the recipe.",
				"items": map[string]any{
					"type": "string",
				},
			},
			"prep_time": map[string]any{
				"type":        "string",
				"description": "Preparation time (e.g. '10 minutes').",
			},
			"cook_time": map[string]any{
				"type":        "string",
				"description": "Cooking time (e.g. '15 minutes').",
			},
			"total_time": map[string]any{
				"type":        "string",
				"description": "Total time required (e.g. '25 minutes').",
			},
			"tags": map[string]any{
				"type":        "array",
				"description": "Keywords or categories for the recipe.",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
		"required": []string{
			"title",
			"description",
			"ingredients",
			"instructions",
			"prep_time",
			"cook_time",
			"total_time",
			"tags",
		},
		"additionalProperties": false,
	}

	model := examples.GetModel("openai", "gpt-4o")

	response, err := model.Generate(context.Background(), &llmsdk.LanguageModelInput{
		SystemPrompt: ptr.To("You are a helpful assistant that extracts structured data from text according to a provided JSON schema."),
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart(text),
			),
		},
		ResponseFormat: llmsdk.NewResponseFormatJSON("recipe", nil, &schema),
	})

	if err != nil {
		panic(err)
	}

	var textPart *llmsdk.TextPart
	for _, part := range response.Content {
		if tp := part.TextPart; tp != nil {
			textPart = tp
			break
		}
	}

	if textPart == nil {
		panic("no text part found in the response")
	}

	var output Output
	if err := json.Unmarshal([]byte(textPart.Text), &output); err != nil {
		panic(err)
	}

	litter.Dump(output)
}
