package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
)

type DelegateParams struct {
	Task string `json:"task"`
}

// Implement the agent delegation pattern, where a main agent delegates tasks
// to sub-agents. The main agent uses the results from the sub-agents'
// execution to make informed decisions and coordinate overall behavior.
type AgentTransferTool[C any] struct {
	agent       *llmagent.Agent[C]
	description string
}

func NewAgentTransferTool[C any](agent *llmagent.Agent[C], description string) *AgentTransferTool[C] {
	return &AgentTransferTool[C]{
		agent:       agent,
		description: description,
	}
}

func (t *AgentTransferTool[C]) Name() string {
	return fmt.Sprintf("transfer_to_%s", t.agent.Name)
}

func (t *AgentTransferTool[C]) Description() string {
	return fmt.Sprintf("Use this tool to transfer the task to %s, which can help with:\n%s",
		t.agent.Name, t.description)
}

func (t *AgentTransferTool[C]) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"task": map[string]any{
				"type": "string",
				"description": `A clear and concise description of the task the agent should achieve. 
Replace any possessive pronouns or ambiguous terms with the actual entity names if possible
so there is enough information for the agent to process without additional context`,
			},
		},
		"required":             []string{"task"},
		"additionalProperties": false,
	}
}

func (t *AgentTransferTool[C]) Execute(ctx context.Context, paramsJSON json.RawMessage, contextVal C, runState *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params DelegateParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	result, err := t.agent.Run(ctx, llmagent.AgentRequest[C]{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart(params.Task, nil),
			),
		},
		Context: contextVal,
	})
	if err != nil {
		return llmagent.AgentToolResult{}, err
	}

	return llmagent.AgentToolResult{
		Content: result.Content,
		IsError: false,
	}, nil
}

type Order struct {
	CustomerName   string
	Address        string
	Quantity       int
	CompletionTime time.Time
}

type MyContext struct {
	mu     *sync.Mutex
	orders []Order
}

func NewMyContext() *MyContext {
	return &MyContext{
		mu:     &sync.Mutex{},
		orders: []Order{},
	}
}

func (c *MyContext) AddOrder(order Order) {
	c.mu.Lock()
	c.orders = append(c.orders, order)
	c.mu.Unlock()
}

func (c *MyContext) GetOrders() []Order {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.orders
}

func (c *MyContext) PruneOrders() {
	c.mu.Lock()
	now := time.Now()
	var remainingOrders []Order
	for _, order := range c.orders {
		if order.CompletionTime.After(now) {
			remainingOrders = append(remainingOrders, order)
		}
	}
	c.orders = remainingOrders
	c.mu.Unlock()
}

type CreateOrderParams struct {
	CustomerName string `json:"customer_name"`
	Address      string `json:"address"`
	Quantity     int    `json:"quantity"`
}

type CreateOrderTool struct{}

func (t *CreateOrderTool) Name() string {
	return "create_order"
}

func (t *CreateOrderTool) Description() string {
	return "Create a new customer order"
}

func (t *CreateOrderTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"customer_name": map[string]any{
				"type": "string",
			},
			"address": map[string]any{
				"type": "string",
			},
			"quantity": map[string]any{
				"type": "integer",
			},
		},
		"required":             []string{"customer_name", "address", "quantity"},
		"additionalProperties": false,
	}
}

func (t *CreateOrderTool) Execute(ctx context.Context, paramsJSON json.RawMessage, context *MyContext, runState *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params CreateOrderParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	fmt.Printf("[order.create_order] Creating order for %s with quantity %d\n",
		params.CustomerName, params.Quantity)

	// Randomly finish between 1 to 10 seconds
	completionDuration := time.Duration(rand.Intn(9)+1) * time.Second
	context.AddOrder(Order{
		CustomerName:   params.CustomerName,
		Address:        params.Address,
		Quantity:       params.Quantity,
		CompletionTime: time.Now().Add(completionDuration),
	})

	result := map[string]string{"status": "creating"}
	resultJSON, _ := json.Marshal(result)

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(string(resultJSON), nil),
		},
		IsError: false,
	}, nil
}

type GetOrdersTool struct{}

func (t *GetOrdersTool) Name() string {
	return "get_orders"
}

func (t *GetOrdersTool) Description() string {
	return "Retrieve the list of customer orders and their status (completed or pending)"
}

func (t *GetOrdersTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": false,
	}
}

type OrderStatus struct {
	CustomerName string `json:"customer_name"`
	Address      string `json:"address"`
	Quantity     int    `json:"quantity"`
	Status       string `json:"status"`
}

func (t *GetOrdersTool) Execute(ctx context.Context, paramsJSON json.RawMessage, contextVal *MyContext, runState *llmagent.RunState) (llmagent.AgentToolResult, error) {
	now := time.Now()

	var result []OrderStatus
	var completedCount int

	for _, order := range contextVal.GetOrders() {
		status := "pending"
		if order.CompletionTime.Before(now) {
			completedCount++
			status = "completed"
		}

		result = append(result, OrderStatus{
			CustomerName: order.CustomerName,
			Address:      order.Address,
			Quantity:     order.Quantity,
			Status:       status,
		})
	}

	fmt.Printf("[order.get_orders] Retrieving orders. Found %d completed orders.\n", completedCount)

	// Remove completed orders
	contextVal.PruneOrders()

	resultJSON, _ := json.Marshal(result)

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(string(resultJSON), nil),
		},
		IsError: false,
	}, nil
}

type DeliverOrderParams struct {
	CustomerName string `json:"customer_name"`
	Address      string `json:"address"`
}

type DeliverOrderTool struct{}

func (t *DeliverOrderTool) Name() string {
	return "deliver_order"
}

func (t *DeliverOrderTool) Description() string {
	return "Deliver a customer order"
}

func (t *DeliverOrderTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"customer_name": map[string]any{
				"type": "string",
			},
			"address": map[string]any{
				"type": "string",
			},
		},
		"required":             []string{"customer_name", "address"},
		"additionalProperties": false,
	}
}

func (t *DeliverOrderTool) Execute(ctx context.Context, paramsJSON json.RawMessage, context *MyContext, runState *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params DeliverOrderParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	fmt.Printf("[delivery.deliver_order] Delivering order for %s to %s\n",
		params.CustomerName, params.Address)

	result := map[string]string{"status": "delivering"}
	resultJSON, _ := json.Marshal(result)

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(string(resultJSON), nil),
		},
		IsError: false,
	}, nil
}

func main() {
	godotenv.Load("../.env")

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable must be set")
	}

	model := openai.NewOpenAIModel(openai.OpenAIModelOptions{
		APIKey:  apiKey,
		ModelID: "gpt-4o",
	})

	// Order processing agent
	orderInst := "You are an order processing agent. Your job is to handle customer orders efficiently and accurately."
	orderAgent := llmagent.NewAgent("order", model,
		llmagent.WithInstructions(
			llmagent.InstructionParam[*MyContext]{String: &orderInst},
		),
		llmagent.WithTools(
			&CreateOrderTool{},
			&GetOrdersTool{},
		),
	)

	// Delivery agent
	deliveryInst := "You are a delivery agent. Your job is to ensure timely and accurate delivery of customer orders."
	deliveryAgent := llmagent.NewAgent("delivery", model,
		llmagent.WithInstructions(
			llmagent.InstructionParam[*MyContext]{String: &deliveryInst},
		),
		llmagent.WithTools(
			&DeliverOrderTool{},
		),
	)

	// Coordinator agent
	coordInst1 := `You are a coordinator agent. Your job is to delegate tasks to the appropriate sub-agents (order processing and delivery) and ensure smooth operation.
You should also poll the order status in every turn to send them for delivery once they are ready.`
	coordInst2 := "Respond by letting me know what you did and what is the result from the sub-agents."
	coordInst3 := `For the purpose of demo:
- you can think of random customer name and address. To be fun, use those from fictions and literatures.
- every time you are called (NEXT), you should randomly create 0 to 1 order.`

	coordinator := llmagent.NewAgent("coordinator", model,
		llmagent.WithInstructions(
			llmagent.InstructionParam[*MyContext]{String: &coordInst1},
			llmagent.InstructionParam[*MyContext]{String: &coordInst2},
			llmagent.InstructionParam[*MyContext]{String: &coordInst3},
		),
		llmagent.WithTools(
			NewAgentTransferTool(orderAgent, "handling customer orders and get order statuses"),
			NewAgentTransferTool(deliveryAgent, "delivering processed orders"),
		),
	)

	contextVal := NewMyContext()

	var messages []llmsdk.Message
	ctx := context.Background()

	// Main loop
	for {
		fmt.Println("\n--- New iteration ---")

		messages = append(messages, llmsdk.NewUserMessage(
			llmsdk.NewTextPart("Next", nil),
		))

		response, err := coordinator.Run(ctx, llmagent.AgentRequest[*MyContext]{
			Messages: messages,
			Context:  contextVal,
		})
		if err != nil {
			log.Fatal(err)
		}

		prettyJSON, err := json.MarshalIndent(response.Content, "", "  ")
		if err != nil {
			log.Fatalf("Failed to format JSON: %v", err)
		}
		fmt.Printf("%s\n", string(prettyJSON))

		// Update messages with the new items
		for _, item := range response.Items {
			if item.Message != nil {
				messages = append(messages, *item.Message)
			}
		}

		// Wait 5 seconds before next iteration
		time.Sleep(5 * time.Second)
	}
}
