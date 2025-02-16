# SearchDataTransporter 搜索中间件服务

一个中间层服务,主要为了支持让本地部署的Ollama框架中运行的大模型实现联网搜索能力

基于Go实现的搜索数据中转服务，提供以下核心功能：

- 🔍 对接外部搜索引擎
- 🚀 异步数据转发至Kafka
- ⚡ 高性能处理（支持100+ QPS）
- 🔒 生产级可靠性保障

## 如何使本地大模型实现联网搜索

[Cherry Studio + Ollama 本地部署 + SearchDataTransporte搜索中间件服务实现联网搜索](/docx/CherryStudio_Ollama本地部署_SearchDataTransporte搜索中间件服务实现联网搜索.md)

## 功能特性

- RESTful API接口
- Kafka消息可靠投递
- 请求速率限制
- 健康检查端点
- 结构化日志
- 优雅关机
- 容器化部署

## 快速开始

### bug与讨论

- 提交bug：https://github.com/aspnmy/SearchDataTransporter/issues
- 提交建议：https://t.me/Ollama_Scanner

### 前置依赖

- Go 1.20+
- Kafka集群（默认本地9092端口）
- 网络可访问百度搜索（测试用）

### 本地运行

```bash
# 克隆仓库
git clone https://github.com/aspnmy/SearchDataTransporter.git
cd SearchDataTransporter

# 安装依赖
go mod download

# 启动服务（默认配置）
KAFKA_BROKERS="localhost:9092" go run main.go
```

### 测试请求

```bash
# 正常搜索请求
curl "http://localhost:8080/search?q=golang"

# 健康检查
curl "http://localhost:8080/health"

# 压力测试（需安装wrk）
wrk -t12 -c400 -d30s "http://localhost:8080/search?q=test"
```

## 配置说明

通过环境变量配置服务参数：


| 环境变量          | 默认值          | 说明             |
| ----------------- | --------------- | ---------------- |
| KAFKA_BROKERS     | localhost:9092  | Kafka集群地址    |
| HTTP_PORT         | 8080            | 服务监听端口     |
| MAX_RESPONSE_SIZE | 10485760 (10MB) | 最大响应体限制   |
| RATE_LIMIT        | 100             | 每秒最大请求数   |
| BURST_LIMIT       | 200             | 突发请求允许数量 |

示例配置：

```bash
export KAFKA_BROKERS="kafka1:9092,kafka2:9092"
export HTTP_PORT=3000
export RATE_LIMIT=200
./SearchDataServer
```

## 生产部署

### Docker部署

```bash
docker build -t search_data_transporter .
docker run -d \
  -p 8080:8080 \
  -e KAFKA_BROKERS="kafka-prod:9092" \
  -e RATE_LIMIT=500 \
  aspnmy/search_data_transporter
```

### Kubernetes示例配置

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: SearchDataTransporter
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: search
        image: aspnmy/search_data_transporter
        env:
        - name: KAFKA_BROKERS
          value: "kafka-cluster:9092"
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
```

## API文档

### 搜索接口

```
GET /search?q={query}

成功响应：
{
  "status": "success",
  "message": "数据已接收"
}

错误响应：
{
  "error": "error_code",
  "message": "错误描述"
}
```

### 健康检查

```
GET /health

健康状态：
{
  "status": "healthy"
}

异常状态：
{
  "error": "kafka_busy",
  "message": "Kafka生产者队列繁忙"
}
```

## 监控指标

内置Prometheus指标端点（需自行配置采集）：

- http_requests_total：请求计数器
- http_response_time_seconds：响应时间分布
- kafka_messages_sent：消息发送统计

## 性能优化

1. Kafka生产者配置：

   - 消息压缩：Snappy
   - 自动重试（3次）
   - 异步批量发送
2. HTTP服务：

   - 连接池复用
   - 超时控制
   - 内存限制

## 常见问题

Q: 如何验证Kafka消息是否发送成功？
A: 查看服务日志或使用kafka-console-consumer消费目标topic

Q: 遇到"请求过于频繁"错误怎么办？
A: 1. 降低请求频率 2. 调整RATE_LIMIT/BURST_LIMIT参数

Q: 如何扩展处理能力？
A: 1. 水平扩展服务实例 2. 优化Kafka分区策略

## 架构图

```
+----------+     +---------------+     +------------+
|  Client  | --> | Search Service| --> | Kafka      |
+----------+     +---------------+     +------------+
                     | 异步处理
                     v
               +------------+
               | 搜索引擎API |
               +------------+
```

[^建议在生产环境部署负载均衡和监控系统]
