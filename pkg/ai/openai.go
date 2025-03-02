package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"watchAlert/config"
	"watchAlert/pkg/tools"
)

type (

	// OpenaiClient 客户端结构
	OpenaiClient struct {
		url     string
		apiKey  string
		timeout time.Duration
		Model   string
		Timeout int
		Stream  bool
	}

	OpenaiRequest struct {
		Model       string           `json:"model"`
		Messages    []*OpenaiMessage `json:"messages"`
		Stream      bool             `json:"stream,omitempty"`
		MaxTokens   int              `json:"max_tokens,omitempty"`
		Temperature float64          `json:"temperature,omitempty"`
	}

	// OpenaiMessage 消息结构
	OpenaiMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	// OpenaiResponse 响应结构
	OpenaiResponse struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	// OpenaiStreamChunk 流式响应块
	OpenaiStreamChunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
)

// NewOpenAIClient 工厂方法
func NewOpenAIClient(config *config.OpenAIConfig) AiClient {
	return &OpenaiClient{
		url:    config.Url,
		apiKey: config.AppKey,
	}
}

func (o *OpenaiClient) ChatCompletion(_ context.Context, prompt string) (string, error) {
	// 构造请求消息
	messages := []*OpenaiMessage{
		{Role: "user", Content: prompt},
	}
	// 组装请求参数
	reqParams := OpenaiRequest{
		Model:     o.Model,
		Messages:  messages,
		Stream:    false,
		MaxTokens: 2048,
	}

	bodyBytes, _ := json.Marshal(reqParams)
	headers := make(map[string]string)
	headers["Authorization"] = "Bearer " + o.apiKey
	response, err := tools.Post(headers, o.url, bytes.NewReader(bodyBytes), int(o.timeout))
	if err != nil {
		return "", err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(response.Body)
		var errResp OpenaiResponse
		json.Unmarshal(errorBody, &errResp)
		return "", fmt.Errorf("OpenAI API错误: %d - %s", response.StatusCode, errResp.Error.Message)
	}

	// 解析响应
	var result OpenaiResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("响应解析失败: %w", err)
	}

	// 检查有效响应
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("无有效返回内容")
	}

	return result.Choices[0].Message.Content, nil
}

func (o *OpenaiClient) StreamCompletion(ctx context.Context, prompt string) (<-chan string, error) {
	reqParams := OpenaiRequest{
		Model:    "gpt-3.5-turbo",
		Messages: []*OpenaiMessage{{Role: "user", Content: prompt}},
		Stream:   o.Stream,
	}
	bodyBytes, _ := json.Marshal(reqParams)
	headers := make(map[string]string)
	headers["Authorization"] = "Bearer " + o.apiKey

	response, err := tools.Post(headers, o.url, bytes.NewReader(bodyBytes), int(o.timeout))
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

func (o *OpenaiClient) Check(_ context.Context) error {
	if o.url == "" || o.apiKey == "" {
		return fmt.Errorf("OpenAI API配置错误")
	}
	return nil
}
