package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/confluentinc/confluent-kago-go/kafka"
	"golang.org/x/time/rate"
)

// 配置结构体
type Config struct {
	KafkaBrokers    []string `env:"KAFKA_BROKERS" envDefault:"localhost:9092"`
	HTTPPort        int      `env:"HTTP_PORT" envDefault:"8080"`
	MaxResponseSize int64    `env:"MAX_RESPONSE_SIZE" envDefault:"10485760"` // 10MB
	RateLimit       int      `env:"RATE_LIMIT" envDefault:"100"`
	BurstLimit      int      `env:"BURST_LIMIT" envDefault:"200"`
}

var (
	cfg         *Config
	httpClient  *http.Client
	kafkaProducer *kafka.Producer
	once        sync.Once
)

// 初始化配置
func init() {
	var err error
	cfg, err = LoadConfig()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化HTTP客户端
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     30 * time.Second,
		},
	}
}

func LoadConfig() (*Config, error) {
	var config Config
	if err := env.Parse(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

// Kafka初始化
func initKafka() error {
	var err error
	once.Do(func() {
		kafkaProducer, err = kafka.NewProducer(&kafka.ConfigMap{
			"bootstrap.servers":   cfg.KafkaBrokers,
			"compression.type":    "snappy",
			"message.timeout.ms":  3000,
			"retries":             3,
			"acks":                "all",
		})
	})
	return err
}

// HTTP处理函数
func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		respondWithError(w, http.StatusBadRequest, "missing_query_param", "缺少查询参数q")
		return
	}

	result, err := searchExternalEngine(query)
	if err != nil {
		log.Printf("搜索失败: %v", err)
		respondWithError(w, http.StatusInternalServerError, "search_failed", "搜索服务暂时不可用")
		return
	}

	if err := sendToKafka("search_results", result); err != nil {
		log.Printf("Kafka发送失败: %v", err)
		respondWithError(w, http.StatusInternalServerError, "kafka_error", "数据处理失败")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "数据已接收",
	})
}

// 外部搜索引擎查询
func searchExternalEngine(query string) ([]byte, error) {
	req, err := http.NewRequest("GET", "https://www.baidu.com/s", nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	q := req.URL.Query()
	q.Add("wd", query)
	req.URL.RawQuery = q.Encode()

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("非200状态码: %d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, cfg.MaxResponseSize))
}

// 发送到Kafka
func sendToKafka(topic string, data []byte) error {
	if err := initKafka(); err != nil {
		return fmt.Errorf("初始化Kafka失败: %w", err)
	}

	deliveryChan := make(chan kafka.Event)
	defer close(deliveryChan)

	err := kafkaProducer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          data,
		Headers:        []kafka.Header{{Key: "source", Value: []byte("search-service")}},
	}, deliveryChan)

	if err != nil {
		return fmt.Errorf("发送消息失败: %w", err)
	}

	select {
	case e := <-deliveryChan:
		msg := e.(*kafka.Message)
		if msg.TopicPartition.Error != nil {
			return msg.TopicPartition.Error
		}
	case <-time.After(5 * time.Second):
		return fmt.Errorf("等待Kafka响应超时")
	}
	return nil
}

// 中间件
func rateLimit(next http.HandlerFunc) http.HandlerFunc {
	limiter := rate.NewLimiter(rate.Limit(cfg.RateLimit), cfg.BurstLimit)
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			respondWithError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "请求过于频繁")
			return
		}
		next(w, r)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{w, http.StatusOK}

		defer func() {
			log.Printf(
				"[%s] %s %d %s %s",
				r.Method,
				r.URL.Path,
				lrw.statusCode,
				time.Since(start),
				r.UserAgent(),
			)
		}()

		next.ServeHTTP(lrw, r)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// 响应处理
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func respondWithError(w http.ResponseWriter, code int, errorCode string, message string) {
	respondWithJSON(w, code, map[string]interface{}{
		"error":   errorCode,
		"message": message,
	})
}

func main() {
	// 初始化Kafka
	if err := initKafka(); err != nil {
		log.Fatalf("初始化Kafka失败: %v", err)
	}

	router := http.NewServeMux()
	router.HandleFunc("/search", rateLimit(searchHandler))
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if kafkaProducer.Len() > 1000 {
			respondWithError(w, http.StatusServiceUnavailable, "kafka_busy", "Kafka生产者队列繁忙")
			return
		}
		respondWithJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
	})

	// 包装中间件
	handler := loggingMiddleware(router)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	// 优雅关机处理
	done := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP服务关闭错误: %v", err)
		}
		close(done)
	}()

	log.Printf("服务启动，监听端口: %d", cfg.HTTPPort)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP服务启动失败: %v", err)
	}
	<-done
	log.Println("服务已正常关闭")
}
