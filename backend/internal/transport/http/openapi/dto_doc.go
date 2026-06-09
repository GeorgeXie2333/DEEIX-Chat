package openapi

// ChatCompletionRequestDoc documents the OpenAI-compatible chat completion body.
// The runtime handler accepts transparent OpenAI-compatible JSON; these fields
// document the supported tool/function calling subset.
type ChatCompletionRequestDoc struct {
	Model             string              `json:"model" example:"gpt-5"`
	Messages          []ChatMessageDoc    `json:"messages"`
	Stream            bool                `json:"stream,omitempty"`
	Tools             []ChatToolDoc       `json:"tools,omitempty"`
	ToolChoice        interface{}         `json:"tool_choice,omitempty" swaggertype:"object"`
	ParallelToolCalls *bool               `json:"parallel_tool_calls,omitempty"`
	Functions         []LegacyFunctionDoc `json:"functions,omitempty"`
	FunctionCall      interface{}         `json:"function_call,omitempty" swaggertype:"object"`
	Temperature       *float64            `json:"temperature,omitempty"`
	// Official OpenAI Chat Completions routes drop reasoning_effort when function tools are present because upstream rejects that combination.
	ReasoningEffort string `json:"reasoning_effort,omitempty" example:"medium"`
	// Legacy OpenAI-compatible output token limit. Official OpenAI Chat Completions routes normalize this to max_completion_tokens; Gemini routes normalize it to max_output_tokens.
	MaxTokens *int `json:"max_tokens,omitempty"`
	// OpenAI Chat Completions output token limit.
	MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`
	// Gemini output token limit.
	MaxOutputTokens *int                   `json:"max_output_tokens,omitempty"`
	Extra           map[string]interface{} `json:"-" swaggerignore:"true"`
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

// ModelListResponseDoc documents GET /v1/models.
type ModelListResponseDoc struct {
	Success bool           `json:"success" example:"true"`
	Data    []ModelItemDoc `json:"data"`
	Object  string         `json:"object" example:"list"`
}

type ModelItemDoc struct {
	ID                     string   `json:"id" example:"gpt-5"`
	Object                 string   `json:"object" example:"model"`
	Created                int64    `json:"created" example:"1626777600"`
	OwnedBy                string   `json:"owned_by" example:"deeix"`
	SupportedEndpointTypes []string `json:"supported_endpoint_types" example:"openai"`
}
