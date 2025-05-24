package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/sashabaranov/go-openai"
)

type Config struct {
	FilePath     string
	SourceBranch string
	TargetBranch string
	Model        string
	Provider     string
	APIKey       string
}

func main() {
	// Display the CLI version
	fmt.Println("Safeguard - Code Change Analysis Tool v1.0.0")
	cfg := parseFlags()

	if cfg.FilePath == "" || cfg.SourceBranch == "" || cfg.TargetBranch == "" {
		fmt.Println("Error: File path, source branch, and target branch are required")
		os.Exit(1)
	}

	sourceContent, err := getFileFromBranch(cfg.FilePath, cfg.SourceBranch)
	if err != nil {
		fmt.Printf("Error getting file from source branch: %v\n", err)
		os.Exit(1)
	}

	targetContent, err := getFileFromBranch(cfg.FilePath, cfg.TargetBranch)
	if err != nil {
		fmt.Printf("Error getting file from target branch: %v\n", err)
		os.Exit(1)
	}

	diff, err := generateDiff(sourceContent, targetContent, cfg.SourceBranch, cfg.TargetBranch, cfg.FilePath)
	if err != nil {
		fmt.Printf("Error generating diff: %v\n", err)
		os.Exit(1)
	}
	
	// Display diff summary for verification
	fmt.Println("\nDiff generated successfully.")
	fmt.Printf("Diff length: %d characters\n", len(diff))

	prompt := buildPrompt(cfg.FilePath, diff)

	var analysis string
	switch cfg.Provider {
	case "anthropic":
		analysis, err = getAnthropicAnalysis(cfg.APIKey, cfg.Model, prompt)
	case "openai":
		analysis, err = getOpenAIAnalysis(cfg.APIKey, cfg.Model, prompt)
	default:
		fmt.Println("Error: Unknown provider. Use 'anthropic' or 'openai'")
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("Error getting analysis: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n--- Analysis of potential bugs ---")
	fmt.Println(analysis)
}

func parseFlags() Config {
	var cfg Config

	flag.StringVar(&cfg.FilePath, "file", "", "Path to the file to analyze")
	flag.StringVar(&cfg.SourceBranch, "source", "", "Source branch")
	flag.StringVar(&cfg.TargetBranch, "target", "", "Target branch")
	flag.StringVar(&cfg.Model, "model", "", "Model to use (claude-3-opus-20240229 for Anthropic, gpt-4-turbo for OpenAI)")
	flag.StringVar(&cfg.Provider, "provider", "anthropic", "LLM provider (anthropic or openai)")
	flag.StringVar(&cfg.APIKey, "key", "", "API key for the provider")

	flag.Parse()

	// Set default models if not provided
	if cfg.Model == "" {
		if cfg.Provider == "anthropic" {
			// Use newer models with correct format
			cfg.Model = "claude-3-5-sonnet-20240620"
			// Other options:
			// cfg.Model = "claude-3-haiku-20240307" 
			// cfg.Model = "claude-3-sonnet-20240229"
			// cfg.Model = "claude-3-opus-20240229"
		} else {
			cfg.Model = "gpt-4-turbo"
		}
	}
	
	// Print the selected model
	fmt.Printf("Using model: %s\n", cfg.Model)

	// Check for API key in env var if not provided
	if cfg.APIKey == "" {
		switch cfg.Provider {
		case "anthropic":
			cfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
			if cfg.APIKey == "" {
				fmt.Println("Error: ANTHROPIC_API_KEY environment variable not set. Use --key flag or set the environment variable.")
				os.Exit(1)
			}
		case "openai":
			cfg.APIKey = os.Getenv("OPENAI_API_KEY")
			if cfg.APIKey == "" {
				fmt.Println("Error: OPENAI_API_KEY environment variable not set. Use --key flag or set the environment variable.")
				os.Exit(1)
			}
		}
	}

	return cfg
}

func getFileFromBranch(filePath, branch string) (string, error) {
	// Print the branch and file we're fetching for debugging
	fmt.Printf("Fetching file '%s' from branch '%s'\n", filePath, branch)
	// Expand home directory if path starts with ~
	if strings.HasPrefix(filePath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		filePath = strings.Replace(filePath, "~", home, 1)
	}

	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", branch, filePath))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git show failed: %w - %s", err, string(output))
	}
	return string(output), nil
}

func generateDiff(sourceContent, targetContent string, sourceBranch, targetBranch, filePath string) (string, error) {
	// Create temporary files
	sourceFile, err := os.CreateTemp("", "source-*")
	if err != nil {
		return "", fmt.Errorf("failed to create source temp file: %w", err)
	}
	defer os.Remove(sourceFile.Name())
	defer sourceFile.Close()
	
	targetFile, err := os.CreateTemp("", "target-*")
	if err != nil {
		return "", fmt.Errorf("failed to create target temp file: %w", err)
	}
	defer os.Remove(targetFile.Name())
	defer targetFile.Close()
	
	// Write content to temp files
	if _, err := sourceFile.WriteString(sourceContent); err != nil {
		return "", fmt.Errorf("failed to write to source temp file: %w", err)
	}
	
	if _, err := targetFile.WriteString(targetContent); err != nil {
		return "", fmt.Errorf("failed to write to target temp file: %w", err)
	}
	
	// Close files to ensure content is flushed to disk
	sourceFile.Close()
	targetFile.Close()
	
	// Run diff on the temp files with labeled headers
	cmd := exec.Command("diff", "-u", 
		"--label", fmt.Sprintf("%s:%s", sourceBranch, filePath), 
		"--label", fmt.Sprintf("%s:%s", targetBranch, filePath), 
		sourceFile.Name(), targetFile.Name())
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	
	if err := cmd.Run(); err != nil {
		// diff exits with status 1 if there are differences, which is expected
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() > 1 {
			return "", fmt.Errorf("diff command failed: %w", err)
		}
	}
	
	return stdout.String(), nil
}

func buildPrompt(filePath string, diff string) string {
	template := "You are an expert code reviewer specializing in finding bugs. Analyze the following changes in the file %s to identify potential bugs, logic errors, edge cases, and performance issues.\n\nDiff:\n```\n%s\n```\n\nFocus on:\n1. Logic errors\n2. Race conditions\n3. Memory leaks\n4. Security vulnerabilities\n5. API contract violations\n6. Edge cases\n7. Performance issues\n\nProvide a concise analysis listing only potential issues. If there are no issues, state that explicitly."
	return fmt.Sprintf(template, filePath, diff)
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicRequest struct {
	Model     string            `json:"model"`
	MaxTokens int               `json:"max_tokens"`
	System    string            `json:"system"`
	Messages  []AnthropicMessage `json:"messages"`
}

type AnthropicResponse struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Role     string `json:"role"`
	Content  []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model    string `json:"model"`
	StopReason string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
}

func getAnthropicAnalysis(apiKey, model, prompt string) (string, error) {
	// Print the starting point for debugging
	fmt.Println("\nStarting Anthropic API analysis...")
	if apiKey == "" {
		return "", fmt.Errorf("Anthropic API key is required")
	}

	reqBody := AnthropicRequest{
		Model:     model,
		MaxTokens: 1024,
		System:    "You are an expert at identifying potential bugs in code changes. Be concise and focus only on likely issues.",
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	// Use the current API version
	req.Header.Set("anthropic-version", "2023-06-01")
	
	// Print request body for debugging
	fmt.Printf("\nRequest payload: %s\n", string(jsonData))
	// Debug API request
	fmt.Println("\nSending request to Anthropic API...")
	fmt.Println("Headers:")
	for k, v := range req.Header {
		fmt.Printf("  %s: %s\n", k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("API Error: Status %d\nResponse: %s\n", resp.StatusCode, string(bodyBytes))
		return "", fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var result AnthropicResponse
	bodyBytes, _ := io.ReadAll(resp.Body)
	fmt.Printf("Response body: %s\n", string(bodyBytes))
	
	// Reset the response body for JSON decoding
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w - response body: %s", err, string(bodyBytes))
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response from Anthropic")
	}

	return result.Content[0].Text, nil
}

func getOpenAIAnalysis(apiKey, model, prompt string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("OpenAI API key is required")
	}

	client := openai.NewClient(apiKey)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are an expert at identifying potential bugs in code changes. Be concise and focus only on likely issues.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			MaxTokens: 1024,
		},
	)

	if err != nil {
		return "", fmt.Errorf("failed to get OpenAI analysis: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}