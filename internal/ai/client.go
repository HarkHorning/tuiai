package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []interface{} `json:"tools,omitempty"`
	Stream   bool      `json:"stream"`
}

type ChatResponse struct {
	Message Message `json:"message"`
}

type Client struct {
	ModelName string
	Endpoint  string
	History   []Message
}

func NewClient(modelName string) *Client {
	if modelName == "" {
		modelName = "tuiai-model"
	}
	return &Client{
		ModelName: modelName,
		Endpoint:  "http://127.0.0.1:11434/api/chat",
		History:   []Message{},
	}
}

func (c *Client) AddMessage(role, content string) {
	c.History = append(c.History, Message{Role: role, Content: content})
}

func (c *Client) GetToolsDefinition() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "list_files",
				"description": "List all markdown (.md) files in the workspace directory.",
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "read_file",
				"description": "Read the contents of a specific markdown (.md) file.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"filename": map[string]interface{}{
							"type":        "string",
							"description": "The relative path of the markdown file to read.",
						},
					},
					"required": []string{"filename"},
				},
			},
		},
		map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "write_file",
				"description": "Create or update a markdown (.md) file with specific content.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"filename": map[string]interface{}{
							"type":        "string",
							"description": "The relative path of the markdown file to write.",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "The complete markdown content to write into the file.",
						},
					},
					"required": []string{"filename", "content"},
				},
			},
		},
		map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "delete_file",
				"description": "Delete a specific markdown (.md) file.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"filename": map[string]interface{}{
							"type":        "string",
							"description": "The relative path of the markdown file to delete.",
						},
					},
					"required": []string{"filename"},
				},
			},
		},
	}
}

func (c *Client) SendChat() (Message, error) {
	reqBody := ChatRequest{
		Model:    c.ModelName,
		Messages: c.History,
		Tools:    c.GetToolsDefinition(),
		Stream:   false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return Message{}, err
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Post(c.Endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return Message{}, fmt.Errorf("failed to connect to local ollama instance (is ollama running on 127.0.0.1:11434?): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Message{}, fmt.Errorf("ollama returned non-200 status code: %d", resp.StatusCode)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return Message{}, err
	}

	return chatResp.Message, nil
}
