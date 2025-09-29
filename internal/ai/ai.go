package ai

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/packages/param"
	"google.golang.org/genai"
)

func CallGPT(systemMessage string, userMessage string, maxTokens param.Opt[int64], temperature param.Opt[float64]) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")

	if apiKey == "" {
		return "", ErrAPIKeyNotSet
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	res, err := client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT3_5Turbo,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemMessage),
			openai.UserMessage(userMessage),
		},
		MaxTokens:   maxTokens,
		Temperature: temperature,
	})

	if err != nil {
		return "", err
	}

	if len(res.Choices) == 0 {
		return "", ErrNoResponse
	}

	return res.Choices[0].Message.Content, nil

}

func CallGemini(systemMessage string, userMessage string, maxTokens int32, temperature float32) (string, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")

	client, err := genai.NewClient(context.TODO(), &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return "", err
	}

	parts := []*genai.Part{
		{
			Text: systemMessage,
		},
		{
			Text: userMessage,
		},
	}
	modelConfig := genai.GenerateContentConfig{Temperature: &temperature, MaxOutputTokens: maxTokens}

	result, err := client.Models.GenerateContent(context.TODO(), "gemini-2.0-flash", []*genai.Content{
		{
			Parts: parts,
		},
	}, &modelConfig)
	if err != nil {
		return "", err
	}

	if len(result.Candidates) == 0 {
		return "", ErrNoResponse
	}

	return result.Candidates[0].Content.Parts[0].Text, nil

}

func CallOllama(systemMessage string, userMessage string) (string, error) {
	// apiPath := "/usr/local/bin/ollama"
	apiPath := os.Getenv("OLLAMA_API_PATH")

	if apiPath == "" {
		return "", fmt.Errorf("ollama binary not found in PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prompt := strings.Join([]string{systemMessage, userMessage}, "\n\n")

	cmd := exec.CommandContext(ctx, apiPath, "run", "llama3.1:8b", prompt)

	out, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("ollama command timed out")
	}

	if err != nil {
		return "", fmt.Errorf("ollama command failed: %v, output: %s", err, string(out))
	}

	return string(out), nil

}

func GenerateCommitMessage(diff string, status string) (string, error) {
	systemMessage := "You are a highly skilled software engineer with deep expertise in crafting precise, professional, and conventional git commit messages. Given a git diff and status, generate a single, clear, and accurate commit message that succinctly summarizes the intent and scope of the changes. Only output the commit message itself, with no explanations, prefixes, formatting, or any other text. The output must be ready to use as a commit message and strictly adhere to best practices."

	// TODO: Remove whitespaces from diff and status to save tokens

	userMessage := "diff: " + diff + "\n\nstatus: " + status
	// maxTokens := param.NewOpt[int64](60)
	// temperature := param.NewOpt(0.7)

	// return CallGPT(systemMessage, userMessage, maxTokens, temperature)
	// return CallOllama(systemMessage, userMessage)
	return CallGemini(systemMessage, userMessage, 256, 0.7)
}
