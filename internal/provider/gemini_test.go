package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewGemini_EmptyAPIKey(t *testing.T) {
	_, err := NewGemini("")
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("expected 'API key is required', got %q", err.Error())
	}
}

func TestNewGemini_ValidAPIKey(t *testing.T) {
	g, err := NewGemini("test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g == nil {
		t.Fatal("expected non-nil Gemini")
	}
}

func TestGemini_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{
							{"text": "Hello from Gemini"},
						},
						"role": "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     10,
				"candidatesTokenCount": 5,
				"totalTokenCount":      15,
			},
			"modelVersion": "gemini-2.0-flash",
		})
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	resp, err := g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		System:   "You are helpful.",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello from Gemini" {
		t.Errorf("expected 'Hello from Gemini', got %q", resp.Content)
	}
	if resp.Model != "gemini-2.0-flash" {
		t.Errorf("expected model 'gemini-2.0-flash', got %q", resp.Model)
	}
	if resp.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.InputTokens)
	}
	if resp.OutputTokens != 5 {
		t.Errorf("expected 5 output tokens, got %d", resp.OutputTokens)
	}
	if resp.StopReason != "stop" {
		t.Errorf("expected stop reason 'stop', got %q", resp.StopReason)
	}
}

func TestGemini_Send_RequestFormat(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotCT string
	var gotMsgCount int
	var gotFirstRole string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("x-goog-api-key")
		gotCT = r.Header.Get("Content-Type")

		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		_ = json.Unmarshal(body, &req)

		// Model is in the URL path, not body
		if contents, ok := req["contents"].([]interface{}); ok {
			gotMsgCount = len(contents)
			if len(contents) >= 1 {
				if first, ok := contents[0].(map[string]interface{}); ok {
					gotFirstRole, _ = first["role"].(string)
				}
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{{"text": "ok"}},
						"role":  "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     1,
				"candidatesTokenCount": 1,
			},
		})
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		System:   "Be helpful",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/models/gemini-2.0-flash:generateContent" {
		t.Errorf("expected /models/gemini-2.0-flash:generateContent, got %s", gotPath)
	}
	if gotAuth != "test-key" {
		t.Errorf("expected 'test-key', got %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("expected 'application/json', got %q", gotCT)
	}
	if gotMsgCount != 1 {
		t.Fatalf("expected 1 content (user only, system in systemInstruction), got %d", gotMsgCount)
	}
	if gotFirstRole != "user" {
		t.Errorf("expected first content role 'user', got %v", gotFirstRole)
	}
}

func TestGemini_Send_SystemInstruction(t *testing.T) {
	var gotSystemInstruction map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		_ = json.Unmarshal(body, &req)

		if si, ok := req["systemInstruction"].(map[string]interface{}); ok {
			gotSystemInstruction = si
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{{"text": "ok"}},
						"role":  "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     1,
				"candidatesTokenCount": 1,
			},
		})
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		System:   "Be helpful",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotSystemInstruction == nil {
		t.Fatal("expected systemInstruction in request")
	}

	parts, ok := gotSystemInstruction["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		t.Fatal("expected systemInstruction.parts to be non-empty array")
	}

	firstPart, ok := parts[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected first part to be a map")
	}

	text, ok := firstPart["text"].(string)
	if !ok || text != "Be helpful" {
		t.Errorf("expected systemInstruction.parts[0].text = 'Be helpful', got %v", text)
	}
}

func TestGemini_Send_OmitsEmptySystem(t *testing.T) {
	var hasSystemInstruction bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		_ = json.Unmarshal(body, &req)

		_, hasSystemInstruction = req["systemInstruction"]

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{{"text": "ok"}},
						"role":  "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     1,
				"candidatesTokenCount": 1,
			},
		})
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hasSystemInstruction {
		t.Error("expected systemInstruction to be omitted when empty")
	}
}

func TestGemini_Send_OmitsZeroTemperature(t *testing.T) {
	var hasGenerationConfig bool
	var hasTemperature bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		_ = json.Unmarshal(body, &req)

		if gc, ok := req["generationConfig"].(map[string]interface{}); ok {
			hasGenerationConfig = true
			_, hasTemperature = gc["temperature"]
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{{"text": "ok"}},
						"role":  "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     1,
				"candidatesTokenCount": 1,
			},
		})
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{
		Model:       "gemini-2.0-flash",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasGenerationConfig && hasTemperature {
		t.Error("expected temperature to be omitted when 0")
	}
}

func TestGemini_Send_OmitsZeroMaxTokens(t *testing.T) {
	var hasMaxOutputTokens bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		_ = json.Unmarshal(body, &req)

		if gc, ok := req["generationConfig"].(map[string]interface{}); ok {
			_, hasMaxOutputTokens = gc["maxOutputTokens"]
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{{"text": "ok"}},
						"role":  "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     1,
				"candidatesTokenCount": 1,
			},
		})
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{
		Model:     "gemini-2.0-flash",
		Messages:  []Message{{Role: "user", Content: "Hi"}},
		MaxTokens: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasMaxOutputTokens {
		t.Error("expected maxOutputTokens to be omitted when 0")
	}
}

func TestGemini_Send_IncludesMaxTokens(t *testing.T) {
	var gotMaxTokens float64
	var hasMaxTokens bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		_ = json.Unmarshal(body, &req)

		if gc, ok := req["generationConfig"].(map[string]interface{}); ok {
			if mot, ok := gc["maxOutputTokens"]; ok {
				hasMaxTokens = true
				gotMaxTokens = mot.(float64)
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{{"text": "ok"}},
						"role":  "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     1,
				"candidatesTokenCount": 1,
			},
		})
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{
		Model:     "gemini-2.0-flash",
		Messages:  []Message{{Role: "user", Content: "Hi"}},
		MaxTokens: 4096,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasMaxTokens {
		t.Error("expected maxOutputTokens to be present")
	}
	if gotMaxTokens != 4096 {
		t.Errorf("expected maxOutputTokens = 4096, got %v", gotMaxTokens)
	}
}

func TestGemini_Send_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"code":401,"message":"API key not valid.","status":"UNAUTHENTICATED"}}`))
	}))
	defer server.Close()

	g, _ := NewGemini("bad-key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryAuth {
		t.Errorf("expected ErrCategoryAuth, got %s", provErr.Category)
	}
}

func TestGemini_Send_ForbiddenError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{Model: "gemini-2.0-flash", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryAuth {
		t.Errorf("expected ErrCategoryAuth, got %s", provErr.Category)
	}
}

func TestGemini_Send_NotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{Model: "gemini-2.0-flash", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryBadRequest {
		t.Errorf("expected ErrCategoryBadRequest, got %s", provErr.Category)
	}
}

func TestGemini_Send_RateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{Model: "gemini-2.0-flash", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryRateLimit {
		t.Errorf("expected ErrCategoryRateLimit, got %s", provErr.Category)
	}
}

func TestGemini_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{Model: "gemini-2.0-flash", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %s", provErr.Category)
	}
}

func TestGemini_Send_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := g.Send(ctx, &Request{Model: "gemini-2.0-flash", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryTimeout {
		t.Errorf("expected ErrCategoryTimeout, got %s", provErr.Category)
	}
}

func TestGemini_Send_EmptyCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []interface{}{},
			"usageMetadata": map[string]int{
				"promptTokenCount":     1,
				"candidatesTokenCount": 0,
			},
		})
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{Model: "gemini-2.0-flash", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %s", provErr.Category)
	}
	if !strings.Contains(provErr.Message, "no candidates") {
		t.Errorf("expected message to contain 'no candidates', got %q", provErr.Message)
	}
}

func TestGemini_Send_ErrorResponseParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"Invalid model specified","status":"INVALID_ARGUMENT"}}`))
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{Model: "bad-model", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if !strings.Contains(provErr.Message, "Invalid model specified") {
		t.Errorf("expected parsed error message, got %q", provErr.Message)
	}
}

func TestGemini_Send_UnparseableErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{Model: "gemini-2.0-flash", Messages: []Message{{Role: "user", Content: "Hi"}}})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Message != "Bad Request" {
		t.Errorf("expected 'Bad Request', got %q", provErr.Message)
	}
}

func TestGemini_Send_WithTools(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{{"text": "ok"}},
						"role":  "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     1,
				"candidatesTokenCount": 1,
			},
		})
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	_, _ = g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{
				Name:        "call_agent",
				Description: "Delegate a task to a sub-agent.",
				Parameters: map[string]ToolParameter{
					"agent":   {Type: "string", Description: "Agent name", Required: true},
					"task":    {Type: "string", Description: "Task description", Required: true},
					"context": {Type: "string", Description: "Additional context", Required: false},
				},
			},
		},
	})

	if _, ok := gotBody["tools"]; !ok {
		t.Fatal("request body does not contain 'tools' key")
	}

	var tools []map[string]json.RawMessage
	if err := json.Unmarshal(gotBody["tools"], &tools); err != nil {
		t.Fatalf("failed to parse tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("len(tools) = %d, want 1", len(tools))
	}

	var fnDecls []map[string]json.RawMessage
	_ = json.Unmarshal(tools[0]["functionDeclarations"], &fnDecls)
	if len(fnDecls) != 1 {
		t.Fatalf("len(functionDeclarations) = %d, want 1", len(fnDecls))
	}

	var name string
	_ = json.Unmarshal(fnDecls[0]["name"], &name)
	if name != "call_agent" {
		t.Errorf("functionDeclarations[0].name = %q, want %q", name, "call_agent")
	}

	var desc string
	_ = json.Unmarshal(fnDecls[0]["description"], &desc)
	if desc != "Delegate a task to a sub-agent." {
		t.Errorf("functionDeclarations[0].description = %q, want %q", desc, "Delegate a task to a sub-agent.")
	}

	var params map[string]json.RawMessage
	_ = json.Unmarshal(fnDecls[0]["parameters"], &params)

	var paramsType string
	_ = json.Unmarshal(params["type"], &paramsType)
	if paramsType != "object" {
		t.Errorf("parameters.type = %q, want %q", paramsType, "object")
	}

	var props map[string]map[string]interface{}
	_ = json.Unmarshal(params["properties"], &props)
	if _, ok := props["agent"]; !ok {
		t.Error("parameters.properties missing 'agent'")
	}
	if _, ok := props["task"]; !ok {
		t.Error("parameters.properties missing 'task'")
	}
	if _, ok := props["context"]; !ok {
		t.Error("parameters.properties missing 'context'")
	}

	var required []string
	_ = json.Unmarshal(params["required"], &required)
	requiredMap := make(map[string]bool)
	for _, r := range required {
		requiredMap[r] = true
	}
	if !requiredMap["agent"] {
		t.Error("required does not include 'agent'")
	}
	if !requiredMap["task"] {
		t.Error("required does not include 'task'")
	}
	if requiredMap["context"] {
		t.Error("required should not include 'context'")
	}
}

func TestGemini_Send_WithoutTools(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{{"text": "ok"}},
						"role":  "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     1,
				"candidatesTokenCount": 1,
			},
		})
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	_, _ = g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools:    nil,
	})

	if _, ok := gotBody["tools"]; ok {
		t.Error("request body should NOT contain 'tools' key when Tools is nil")
	}
}

func TestGemini_Send_ToolCallResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"candidates": [{
				"content": {
					"parts": [{
						"functionCall": {
							"name": "call_agent",
							"args": {"agent": "helper", "task": "run tests"}
						}
					}],
					"role": "model"
				},
				"finishReason": "STOP"
			}],
			"usageMetadata": {"promptTokenCount": 10, "candidatesTokenCount": 5}
		}`
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	resp, err := g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{Name: "call_agent", Description: "test", Parameters: map[string]ToolParameter{
				"agent": {Type: "string", Description: "agent", Required: true},
				"task":  {Type: "string", Description: "task", Required: true},
			}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "call_agent" {
		t.Errorf("ToolCalls[0].Name = %q, want %q", resp.ToolCalls[0].Name, "call_agent")
	}
	if resp.ToolCalls[0].Arguments["agent"] != "helper" {
		t.Errorf("ToolCalls[0].Arguments[agent] = %q, want %q", resp.ToolCalls[0].Arguments["agent"], "helper")
	}
	if resp.ToolCalls[0].Arguments["task"] != "run tests" {
		t.Errorf("ToolCalls[0].Arguments[task] = %q, want %q", resp.ToolCalls[0].Arguments["task"], "run tests")
	}
}

func TestGemini_Send_ToolCallNullContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"candidates": [{
				"content": {
					"parts": [{
						"functionCall": {
							"name": "call_agent",
							"args": {"agent": "a", "task": "b"}
						}
					}],
					"role": "model"
				},
				"finishReason": "STOP"
			}],
			"usageMetadata": {"promptTokenCount": 10, "candidatesTokenCount": 5}
		}`
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	resp, err := g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{Name: "call_agent", Description: "test", Parameters: map[string]ToolParameter{}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "" {
		t.Errorf("Content = %q, want empty string", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
	}
}

func TestGemini_Send_InvalidToolCallArguments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"candidates": [{
				"content": {
					"parts": [{
						"functionCall": {
							"name": "call_agent",
							"args": "invalid"
						}
					}],
					"role": "model"
				},
				"finishReason": "STOP"
			}],
			"usageMetadata": {"promptTokenCount": 10, "candidatesTokenCount": 5}
		}`
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	resp, err := g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{Name: "call_agent", Description: "test", Parameters: map[string]ToolParameter{}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "call_agent" {
		t.Errorf("ToolCalls[0].Name = %q, want %q", resp.ToolCalls[0].Name, "call_agent")
	}
}

func TestGemini_Send_ToolResultMessage(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{{"text": "ok"}},
						"role":  "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     1,
				"candidatesTokenCount": 1,
			},
		})
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	_, _ = g.Send(context.Background(), &Request{
		Model: "gemini-2.0-flash",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "", ToolCalls: []ToolCall{
				{ID: "call_1", Name: "call_agent", Arguments: map[string]string{"agent": "helper"}},
			}},
			{Role: "tool", ToolResults: []ToolResult{
				{CallID: "call_1", Content: "result text", IsError: false},
			}},
		},
	})

	var contents []json.RawMessage
	_ = json.Unmarshal(gotBody["contents"], &contents)

	found := false
	for _, raw := range contents {
		var content map[string]interface{}
		_ = json.Unmarshal(raw, &content)
		if content["role"] == "user" {
			if parts, ok := content["parts"].([]interface{}); ok {
				for _, p := range parts {
					if part, ok := p.(map[string]interface{}); ok {
						if fr, ok := part["functionResponse"].(map[string]interface{}); ok {
							found = true
							if fr["name"] != "call_1" {
								t.Errorf("functionResponse.name = %v, want %q", fr["name"], "call_1")
							}
						}
					}
				}
			}
		}
	}
	if !found {
		t.Error("no functionResponse part found in request")
	}
}

func TestGemini_Send_AssistantToolCallMessage(t *testing.T) {
	var gotBody map[string]json.RawMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{{"text": "ok"}},
						"role":  "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     1,
				"candidatesTokenCount": 1,
			},
		})
	}))
	defer server.Close()

	g, _ := NewGemini("key", WithGeminiBaseURL(server.URL))
	_, _ = g.Send(context.Background(), &Request{
		Model: "gemini-2.0-flash",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "I'll help", ToolCalls: []ToolCall{
				{ID: "call_1", Name: "call_agent", Arguments: map[string]string{"agent": "helper", "task": "work"}},
			}},
			{Role: "tool", ToolResults: []ToolResult{
				{CallID: "call_1", Content: "done", IsError: false},
			}},
		},
	})

	var contents []json.RawMessage
	_ = json.Unmarshal(gotBody["contents"], &contents)

	found := false
	for _, raw := range contents {
		var content map[string]json.RawMessage
		_ = json.Unmarshal(raw, &content)

		var role string
		_ = json.Unmarshal(content["role"], &role)

		if role == "model" {
			if _, ok := content["parts"]; !ok {
				t.Error("model message missing 'parts' field")
				continue
			}
			found = true

			var parts []map[string]json.RawMessage
			_ = json.Unmarshal(content["parts"], &parts)

			foundFuncCall := false
			for _, part := range parts {
				if fc, ok := part["functionCall"]; ok {
					foundFuncCall = true
					var fnCall map[string]json.RawMessage
					_ = json.Unmarshal(fc, &fnCall)

					var name string
					_ = json.Unmarshal(fnCall["name"], &name)
					if name != "call_agent" {
						t.Errorf("functionCall.name = %q, want %q", name, "call_agent")
					}

					var args map[string]interface{}
					_ = json.Unmarshal(fnCall["args"], &args)
					if args["agent"] != "helper" {
						t.Errorf("functionCall.args.agent = %v, want %q", args["agent"], "helper")
					}
					if args["task"] != "work" {
						t.Errorf("functionCall.args.task = %v, want %q", args["task"], "work")
					}
				}
			}
			if !foundFuncCall {
				t.Error("no functionCall part found in model message")
			}
		}
	}
	if !found {
		t.Error("no model message with functionCall found in request")
	}
}

func TestGemini_Send_RoleMappingAssistantToModel(t *testing.T) {
	var gotRoles []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		_ = json.Unmarshal(body, &req)

		if contents, ok := req["contents"].([]interface{}); ok {
			for _, c := range contents {
				if content, ok := c.(map[string]interface{}); ok {
					if role, ok := content["role"].(string); ok {
						gotRoles = append(gotRoles, role)
					}
				}
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{{"text": "ok"}},
						"role":  "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     1,
				"candidatesTokenCount": 1,
			},
		})
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{
		Model: "gemini-2.0-flash",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "Hello!"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotRoles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(gotRoles))
	}
	if gotRoles[0] != "user" {
		t.Errorf("expected first role 'user', got %q", gotRoles[0])
	}
	if gotRoles[1] != "model" {
		t.Errorf("expected second role 'model' (assistant mapped), got %q", gotRoles[1])
	}
}

func TestGemini_Send_CustomBaseURL(t *testing.T) {
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{{"text": "ok"}},
						"role":  "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     1,
				"candidatesTokenCount": 1,
			},
		})
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	_, err := g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != "/models/gemini-2.0-flash:generateContent" {
		t.Errorf("expected /models/gemini-2.0-flash:generateContent, got %s", gotPath)
	}
}
