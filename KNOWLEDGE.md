# Safeguard: Code Knowledge Guide

This document explains the structure and functionality of the Safeguard code change analysis tool.

## Overview

Safeguard is a CLI tool written in Go that leverages Large Language Models (LLMs) to analyze changes between Git branches and identify potential bugs or issues. The tool works in several stages:

1. Retrieve file content from two different Git branches
2. Generate a diff between the two versions
3. Send the diff to an LLM (Anthropic Claude or OpenAI) for analysis
4. Present the analysis results to the user

## Core Components

### Main Module (`main.go`)

The main module handles the CLI command parsing, orchestration, and execution flow:

- **`main()`**: Entry point that initializes the tool and orchestrates the workflow
- **`parseFlags()`**: Parses command line arguments and prepares configuration
- **`getFileFromBranch()`**: Retrieves file content from a specific Git branch
- **`generateDiff()`**: Creates a diff between two file versions
- **`buildPrompt()`**: Constructs the prompt for the LLM
- **`getAnthropicAnalysis()`**: Handles API calls to Anthropic Claude
- **`getOpenAIAnalysis()`**: Handles API calls to OpenAI

### Interactive Module (`interactive.go`)

The interactive module provides a terminal user interface for selecting files:

- **`launchFileSelector()`**: Initializes and runs the file selector UI
- **`listGitFiles()`**: Lists all files in the Git repository
- **`fileModel`**: The BubbleTea model for the file selection interface

## Key Data Structures

### `Config` struct

Stores the configuration for the analysis run:

```go
type Config struct {
	FilePath     string // Path to the file to analyze
	SourceBranch string // Source branch for comparison
	TargetBranch string // Target branch for comparison
	Model        string // LLM model to use
	Provider     string // LLM provider ("anthropic" or "openai")
	APIKey       string // API key for the provider
}
```

### LLM API Structures

Structures for interacting with LLM APIs:

#### Anthropic API

```go
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
	ID          string `json:"id"`
	Type        string `json:"type"`
	Role        string `json:"role"`
	Content     []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model       string `json:"model"`
	StopReason  string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
}
```

## Workflow

1. **Command Line Parsing**:
   - Parse flags for file path, branches, model, provider, etc.
   - If in interactive mode, launch the file selector

2. **File Retrieval**:
   - Use Git commands to retrieve file content from both branches
   - Handle potential errors like non-existent files or branches

3. **Diff Generation**:
   - Create temporary files with the content from each branch
   - Use the `diff` command to generate a unified diff
   - Label the diff with branch and file information

4. **LLM Analysis**:
   - Build a prompt with the diff and instructions
   - Send the prompt to the chosen LLM provider (Anthropic or OpenAI)
   - Handle API authentication and error responses

5. **Result Presentation**:
   - Display the analysis results to the user
   - Format the output for readability

## Interactive Mode

The interactive mode provides a terminal user interface for selecting files:

1. Lists all files tracked by Git in the repository
2. Allows search and filtering with "/" key
3. Enables navigation with arrow keys
4. Permits selection with Enter key

Technically, it uses the BubbleTea library to create a terminal UI that lists all Git-tracked files and allows selection.

## API Integration

### Anthropic Claude

The tool uses the Anthropic Messages API to analyze code changes:

- Sends a structured prompt with the diff and instructions
- Uses a system prompt to guide the model's response
- Processes the JSON response to extract the analysis

### OpenAI

The tool uses the OpenAI Chat Completion API to analyze code changes:

- Uses the official Go client library
- Sends both system and user messages
- Extracts the response content for presentation

## Error Handling

The tool includes robust error handling for various scenarios:

- Missing configuration parameters
- Git command failures
- Diff generation issues
- API authentication errors
- API response processing errors

## Debugging Features

The tool includes several debugging features:

- Verbose output of API requests and responses
- Display of HTTP headers and status codes
- Information about file retrieval and diff generation
- Error messages with context for troubleshooting