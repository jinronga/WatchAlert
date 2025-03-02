package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
	"watchAlert/config"
	"watchAlert/pkg/tools"
)

type (
	// DeepSeekClient is a client for the DeepSeek API.
	DeepSeekClient struct {
		// url
		url string
		// apiKey
		apiKey string
		// timeout
		timeout   time.Duration
		maxTokens int
		model     string
	}

	// DeepSeekRequest API 请求结构
	DeepSeekRequest struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Stream    bool `json:"stream"`
		MaxTokens int  `json:"maxTokens,omitempty"`
	}

	// DeepSeekResponse 标准响应结构
	DeepSeekResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	// DeepSeekStreamChunk 流式响应结构
	DeepSeekStreamChunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
)

// NewDeepSeekClient creates a new DeepSeekClient instance.
func NewDeepSeekClient(config config.DeepSeekConfig) AiClient {
	return &DeepSeekClient{
		url:       config.Url,
		apiKey:    config.AppKey,
		maxTokens: config.MaxTokens,
	}
}

func (d *DeepSeekClient) Check(_ context.Context) error {
	if d.url == "" || d.apiKey == "" {
		return errors.New("DeepSeek API url or apiKey is empty")
	}
	return nil
}

func (d *DeepSeekClient) ChatCompletion(_ context.Context, prompt string) (string, error) {
	reqParam := DeepSeekRequest{
		Model: d.model,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{"user", prompt},
		},
		Stream:    false,
		MaxTokens: d.maxTokens,
	}

	if d.timeout == 0 {
		d.timeout = 30
	}

	bodyReader, _ := json.Marshal(reqParam)

	headers := make(map[string]string)
	headers["Authorization"] = "Bearer " + d.apiKey

	response, err := tools.Post(headers, d.url, bytes.NewReader(bodyReader), int(d.timeout))
	if err != nil {
		return "", err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return "", fmt.Errorf("DeepSeekClient API返回错误: %d %s", response.StatusCode, string(body))
	}

	var result DeepSeekResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("响应解析失败: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("无有效返回内容")
	}

	return result.Choices[0].Message.Content, nil
}

func (d *DeepSeekClient) StreamCompletion(ctx context.Context, prompt string) (<-chan string, error) {
	reqParam := DeepSeekRequest{
		Model: d.model,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{"user", prompt},
		},
		Stream:    true,
		MaxTokens: d.maxTokens,
	}

	if d.timeout == 0 {
		d.timeout = 30
	}

	bodyReader, _ := json.Marshal(reqParam)

	headers := make(map[string]string)
	headers["Authorization"] = "Bearer " + d.apiKey

	response, err := tools.Post(headers, d.url, bytes.NewReader(bodyReader), int(d.timeout))
	if err != nil {
		return nil, fmt.Errorf("流式请求失败: %w", err)
	}

	// 创建流式通道
	streamChan := make(chan string)
	go func() {
		defer close(streamChan)
		defer response.Body.Close()

		// 流式处理
		decoder := json.NewDecoder(response.Body)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var chunk DeepSeekStreamChunk
				if err := decoder.Decode(&chunk); err != nil {
					if err == io.EOF {
						return
					}
					streamChan <- fmt.Sprintf("[ERROR] %v", err)
					return
				}

				if len(chunk.Choices) > 0 {
					content := chunk.Choices[0].Delta.Content
					if content != "" {
						streamChan <- content
					}
				}
			}
		}
	}()
	return streamChan, nil
}
