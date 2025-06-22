import { getModel } from "./get-model.ts";

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
Pour the wet ingredients into the dry and stir until just combined (batter will be lumpy, don’t overmix).
Heat a skillet over medium heat and lightly grease with butter or oil.
Pour 1/4 cup batter for each pancake. Cook until bubbles form on the surface, then flip and cook until golden brown.
Serve warm with maple syrup, fresh fruit, or whipped cream.

Prep time: 10 minutes
Cook time: 15 minutes
Total time: 25 minutes

Tags: breakfast, easy, kid-friendly`;

const schema = {
  title: "Recipe",
  type: "object",
  properties: {
    title: {
      type: "string",
      description: "The name of the recipe.",
    },
    description: {
      type: "string",
      description: "A short description of the recipe.",
    },
    ingredients: {
      type: "array",
      description: "List of ingredients required for the recipe.",
      items: {
        type: "string",
      },
    },
    instructions: {
      type: "array",
      description: "Step-by-step instructions for preparing the recipe.",
      items: {
        type: "string",
      },
    },
    prep_time: {
      type: "string",
      description: "Preparation time (e.g. '10 minutes').",
    },
    cook_time: {
      type: "string",
      description: "Cooking time (e.g. '15 minutes').",
    },
    total_time: {
      type: "string",
      description: "Total time required (e.g. '25 minutes').",
    },
    tags: {
      type: "array",
      description: "Keywords or categories for the recipe.",
      items: {
        type: "string",
      },
    },
  },
  required: [
    "title",
    "description",
    "ingredients",
    "instructions",
    "prep_time",
    "cook_time",
    "total_time",
    "tags",
  ],
  additionalProperties: false,
};

const model = getModel("openai", "gpt-4o");

const response = await model.generate({
  system_prompt: `You are a helpful assistant that extracts structured data from text according to a provided JSON schema.`,
  messages: [
    {
      role: "user",
      content: [{ type: "text", text }],
    },
  ],
  response_format: {
    type: "json",
    name: "recipe",
    schema,
  },
});

const textPart = response.content.find((part) => part.type === "text");
if (!textPart) {
  throw new Error("No text part found in the response");
}

console.log(JSON.parse(textPart.text));
