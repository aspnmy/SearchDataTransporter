package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/ollama/ollama/api"
)

const (
	searchEndpoint   = "http://localhost:8080/search"
	kafkaBrokers     = "localhost:9092"
	searchTopic      = "search_results"
	timeout          = 10 * time.Second
	pollInterval     = 1 * time.Second
)

var (
	consumer *kafka.Consumer
)

type SearchPlugin struct {
	api.Client
}

func init() {
	// 初始化Kafka消费者
	var err error
	consumer, err = kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": kafkaBrokers,
		"group.id":          "ollama-search",
		"auto.offset.reset": "latest",
	})
	
	if err != nil {
		log.Fatalf("Failed to create Kafka consumer: %v", err)
	}
	
	err = consumer.Subscribe(searchTopic, nil)
	if err != nil {
		log.Fatalf("Failed to subscribe to topic: %v", err)
	}
}

func (p *SearchPlugin) Generate(ctx context.Context, req *api.GenerateRequest, fn func(*api.GenerateResponse) error) error {
	// 原始请求上下文保存
	originalStream := func(resp *api.GenerateResponse) error {
		return fn(resp)
	}

	// 发送搜索请求
	searchResp, err := triggerSearch(req.Prompt)
	if err != nil {
		log.Printf("Search failed: %v", err)
		return originalStream(&api.GenerateResponse{
			Response: "当前无法访问网络资源，将使用本地知识回答",
		})
	}

	// 等待并获取搜索结果
	result := waitForSearchResult(ctx, searchResp.SearchID)
	
	// 构造增强后的提示词
	augmentedPrompt := fmt.Sprintf("基于以下网络搜索结果回答问题：\n%s\n\n问题：%s", result, req.Prompt)

	// 创建修改后的请求
	modifiedReq := *req
	modifiedReq.Prompt = augmentedPrompt

	// 调用原始生成逻辑
	return p.Client.Generate(ctx, &modifiedReq, originalStream)
}

func triggerSearch(query string) (*SearchResponse, error) {
	resp, err := http.Get(fmt.Sprintf("%s?q=%s", searchEndpoint, query))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searchResp SearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, err
	}

	return &searchResp, nil
}

func waitForSearchResult(ctx context.Context, searchID string) string {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case <-deadline.C:
			return "（网络查询超时，以下为本地知识回答）"
		default:
			msg, err := consumer.ReadMessage(100 * time.Millisecond)
			if err != nil {
				continue
			}

			var result SearchResult
			if err := json.Unmarshal(msg.Value, &result); err != nil {
				continue
			}

			if result.SearchID == searchID {
				return result.Content
			}
		}
	}
}

type SearchResponse struct {
	SearchID string `json:"search_id"`
}

type SearchResult struct {
	SearchID string `json:"search_id"`
	Content  string `json:"content"`
}

// 插件注册逻辑
func New() api.Plugin {
	return &SearchPlugin{
		Client: api.DefaultClient(),
	}
}
