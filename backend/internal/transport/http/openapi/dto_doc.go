package openapi

// ChatCompletionRequestDoc documents the OpenAI-compatible chat completion body.
// The runtime handler accepts transparent OpenAI-compatible JSON; these fields
// document the supported tool/function calling subset.
type ChatCompletionRequestDoc struct {
	Model             string                 `json:"model" example:"gpt-5"`
	Messages          []ChatMessageDoc       `json:"messages"`
	Stream            bool                   `json:"stream,omitempty"`
	Tools             []ChatToolDoc          `json:"tools,omitempty"`
	ToolChoice        interface{}            `json:"tool_choice,omitempty" swaggertype:"object"`
	ParallelToolCalls *bool                  `json:"parallel_tool_calls,omitempty"`
	Functions         []LegacyFunctionDoc    `json:"functions,omitempty"`
	FunctionCall      interface{}            `json:"function_call,omitempty" swaggertype:"object"`
	Temperature       *float64               `json:"temperature,omitempty"`
	MaxTokens         *int                   `json:"max_tokens,omitempty"`
	Extra             map[string]interface{} `json:"-" swaggerignore:"true"`
}

type ChatMessageDoc struct {
	Role         string                 `json:"role" example:"user"`
	Content      interface{}            `json:"content,omitempty" swaggertype:"object"`
	Name         string                 `json:"name,omitempty"`
	ToolCallID   string                 `json:"tool_call_id,omitempty"`
	ToolCalls    []ChatToolCallDoc      `json:"tool_calls,omitempty"`
	FunctionCall *LegacyFunctionCallDoc `json:"function_call,omitempty"`
}

type ChatToolDoc struct {
	Type     string            `json:"type" example:"function"`
	Function LegacyFunctionDoc `json:"function"`
}

type LegacyFunctionDoc struct {
	Name        string                 `json:"name" example:"get_weather"`
	Description string                 `json:"description,omitempty" example:"Get weather for a city"`
	Parameters  map[string]interface{} `json:"parameters,omitempty" swaggertype:"object"`
}

type ChatToolCallDoc struct {
	ID       string                `json:"id,omitempty" example:"call_1"`
	Type     string                `json:"type" example:"function"`
	Function LegacyFunctionCallDoc `json:"function"`
}

type LegacyFunctionCallDoc struct {
	Name      string `json:"name" example:"get_weather"`
	Arguments string `json:"arguments" example:"{\"city\":\"Paris\"}"`
}

type ChatCompletionResponseDoc struct {
	ID      string                    `json:"id" example:"chatcmpl-openapi"`
	Object  string                    `json:"object" example:"chat.completion"`
	Created int64                     `json:"created"`
	Model   string                    `json:"model" example:"gpt-5"`
	Choices []ChatCompletionChoiceDoc `json:"choices"`
	Usage   map[string]interface{}    `json:"usage,omitempty" swaggertype:"object"`
}

type ChatCompletionChoiceDoc struct {
	Index        int            `json:"index"`
	Message      ChatMessageDoc `json:"message"`
	FinishReason string         `json:"finish_reason" example:"tool_calls"`
}
