package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// defaultGeminiBaseURL is the default Google Gemini API base URL.
	defaultGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"
)

// GeminiOption is a functional option for configuring the Gemini provider.
type GeminiOption func(*Gemini)

// WithGeminiBaseURL sets a custom base URL for the Gemini provider.
func WithGeminiBaseURL(url string) GeminiOption {
	return func(g *Gemini) {
		g.baseURL = url
	}
}

// Gemini implements the Provider interface for the Google Gemini API.
type Gemini struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewGemini creates a new Gemini provider. Returns an error if apiKey is empty.
func NewGemini(apiKey string, opts ...GeminiOption) (*Gemini, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	g := &Gemini{
		apiKey:  apiKey,
		baseURL: defaultGeminiBaseURL,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}

	for _, opt := range opts {
		opt(g)
	}

	return g, nil
}

// geminiRequest is the JSON body sent to the Gemini API.
type geminiRequest struct {
	Contents          []geminiContent          `json:"contents"`
	SystemInstruction *geminiSystemInstruction `json:"systemInstruction,omitempty"`
	Tools             []geminiToolDef          `json:"tools,omitempty"`
	GenerationConfig  *geminiGenerationConfig  `json:"generationConfig,omitempty"`
}

// geminiContent represents a content block in the Gemini API.
type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

// geminiPart represents a single part within a content block.
type geminiPart struct {
	Text             *string                 `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

// geminiFunctionCall represents a function call from the model.
type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// geminiFunctionResponse represents a function result to send to the model.
type geminiFunctionResponse struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

// geminiToolDef represents a tool definition in the Gemini API.
type geminiToolDef struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations"`
}

// geminiFunctionDeclaration represents a single function declaration.
type geminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// geminiGenerationConfig represents generation configuration.
type geminiGenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
}

// geminiSystemInstruction represents the system instruction.
type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

// geminiResponse represents the JSON response from the Gemini API.
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text         string              `json:"text,omitempty"`
				FunctionCall *geminiFunctionCall `json:"functionCall,omitempty"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokens     int `json:"promptTokenCount"`
		CandidatesTokens int `json:"candidatesTokenCount"`
		TotalTokens      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	ModelVersion string `json:"modelVersion"`
}

// geminiErrorResponse represents a Gemini API error response.
type geminiErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// convertToGeminiContents converts provider Messages to Gemini format.
// Maps "assistant" role to "model". Handles tool calls and tool results.
func convertToGeminiContents(msgs []Message) []geminiContent {
	var result []geminiContent
	for _, msg := range msgs {
		if msg.Role == "tool" && len(msg.ToolResults) > 0 {
			// Tool result messages are sent as role "user" with functionResponse parts
			var parts []geminiPart
			for _, tr := range msg.ToolResults {
				parts = append(parts, geminiPart{
					FunctionResponse: &geminiFunctionResponse{
						Name: tr.CallID, // Gemini uses the call ID as the name
						Response: map[string]interface{}{
							"result": tr.Content,
						},
					},
				})
			}
			result = append(result, geminiContent{
				Role:  "user",
				Parts: parts,
			})
		} else if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			// Assistant messages with tool calls need functionCall parts
			var parts []geminiPart
			if msg.Content != "" {
				parts = append(parts, geminiPart{
					Text: &msg.Content,
				})
			}
			for _, tc := range msg.ToolCalls {
				args := make(map[string]interface{})
				for k, v := range tc.Arguments {
					args[k] = v
				}
				argsJSON, _ := json.Marshal(args)
				parts = append(parts, geminiPart{
					FunctionCall: &geminiFunctionCall{
						Name: tc.Name,
						Args: argsJSON,
					},
				})
			}
			result = append(result, geminiContent{
				Role:  "model",
				Parts: parts,
			})
		} else {
			// Standard text message - map role "assistant" to "model"
			role := msg.Role
			if role == "assistant" {
				role = "model"
			}
			content := msg.Content
			result = append(result, geminiContent{
				Role:  role,
				Parts: []geminiPart{{Text: &content}},
			})
		}
	}
	return result
}

// convertToGeminiTools converts provider Tools to Gemini wire format.
func convertToGeminiTools(tools []Tool) []geminiToolDef {
	var declarations []geminiFunctionDeclaration
	for _, tool := range tools {
		properties := make(map[string]interface{})
		var required []string
		for name, param := range tool.Parameters {
			properties[name] = map[string]interface{}{
				"type":        param.Type,
				"description": param.Description,
			}
			if param.Required {
				required = append(required, name)
			}
		}

		params := map[string]interface{}{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			params["required"] = required
		}

		declarations = append(declarations, geminiFunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  params,
		})
	}

	return []geminiToolDef{{
		FunctionDeclarations: declarations,
	}}
}

// Send makes a completion request to the Gemini API.
func (g *Gemini) Send(ctx context.Context, req *Request) (*Response, error) {
	body := geminiRequest{
		Contents: convertToGeminiContents(req.Messages),
	}

	// Add system instruction if present
	if req.System != "" {
		body.SystemInstruction = &geminiSystemInstruction{
			Parts: []geminiPart{{
				Text: &req.System,
			}},
		}
	}

	// Add tools if present
	if len(req.Tools) > 0 {
		body.Tools = convertToGeminiTools(req.Tools)
	}

	// Add generation config if temperature or max tokens are set
	if req.Temperature != 0 || req.MaxTokens != 0 {
		config := &geminiGenerationConfig{}
		if req.Temperature != 0 {
			temp := req.Temperature
			config.Temperature = &temp
		}
		if req.MaxTokens != 0 {
			mt := req.MaxTokens
			config.MaxOutputTokens = &mt
		}
		body.GenerationConfig = config
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := g.baseURL + "/models/" + req.Model + ":generateContent"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-goog-api-key", g.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := g.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &ProviderError{
				Category: ErrCategoryTimeout,
				Message:  ctx.Err().Error(),
				Err:      ctx.Err(),
			}
		}
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  err.Error(),
			Err:      err,
		}
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, g.handleErrorResponse(httpResp.StatusCode, respBody)
	}

	var apiResp geminiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  fmt.Sprintf("failed to parse response: %s", err),
			Err:      err,
		}
	}

	if len(apiResp.Candidates) == 0 {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  "response contains no candidates",
		}
	}

	// Parse content - concatenate text parts and extract function calls
	var content string
	var toolCalls []ToolCall
	for _, part := range apiResp.Candidates[0].Content.Parts {
		if part.Text != "" {
			content += part.Text
		}
		if part.FunctionCall != nil {
			args := make(map[string]string)
			if part.FunctionCall.Args != nil && len(part.FunctionCall.Args) > 0 {
				var rawArgs map[string]interface{}
				if err := json.Unmarshal(part.FunctionCall.Args, &rawArgs); err == nil {
					for k, v := range rawArgs {
						args[k] = fmt.Sprintf("%v", v)
					}
				}
			}
			toolCalls = append(toolCalls, ToolCall{
				Name:      part.FunctionCall.Name,
				Arguments: args,
			})
		}
	}

	// Map finish reason: "STOP" -> "stop", "MAX_TOKENS" -> "max_tokens", etc.
	stopReason := strings.ToLower(apiResp.Candidates[0].FinishReason)
	if stopReason == "max_tokens" {
		stopReason = "max_tokens"
	} else if stopReason == "safety" {
		stopReason = "safety"
	}

	return &Response{
		Content:      content,
		Model:        req.Model,
		InputTokens:  apiResp.UsageMetadata.PromptTokens,
		OutputTokens: apiResp.UsageMetadata.CandidatesTokens,
		StopReason:   stopReason,
		ToolCalls:    toolCalls,
	}, nil
}

// handleErrorResponse maps HTTP error responses to ProviderError.
func (g *Gemini) handleErrorResponse(status int, body []byte) *ProviderError {
	message := http.StatusText(status)
	var errResp geminiErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		message = errResp.Error.Message
	}

	return &ProviderError{
		Category: g.mapStatusToCategory(status),
		Status:   status,
		Message:  message,
	}
}

// mapStatusToCategory maps HTTP status codes to error categories.
func (g *Gemini) mapStatusToCategory(status int) ErrorCategory {
	switch status {
	case 401, 403:
		return ErrCategoryAuth
	case 400, 404:
		return ErrCategoryBadRequest
	case 429:
		return ErrCategoryRateLimit
	case 500, 502, 503:
		return ErrCategoryServer
	default:
		return ErrCategoryServer
	}
}
