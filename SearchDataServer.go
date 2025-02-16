package main

import (
    "fmt"
    "io/ioutil"
    "net/http"
    "os/exec"
    "strings"

    "github.com/confluentinc/confluent-kafka-go/kafka"
)

func installDependency() error {
    out, err := exec.Command("go", "get", "github.com/confluentinc/confluent-kafka-go/kafka").CombinedOutput()
    if err!= nil {
        if strings.Contains(string(out), "already using") {
            return nil
        }
        return fmt.Errorf("failed to install dependency: %v, output: %s", err, out)
    }
    return nil
}

// 模拟对接外部搜索引擎接口
func searchExternalEngine(query string) ([]byte, error) {
    // 这里以百度搜索为例，实际需要根据真实API调整
    url := "https://www.baidu.com/s"
    resp, err := http.Get(url + "?wd=" + query)
    if err!= nil {
        return nil, err
    }
    defer resp.Body.Close()

    return ioutil.ReadAll(resp.Body)
}

// 将数据发送到Kafka
func sendToKafka(topic string, data []byte) error {
    p, err := kafka.NewProducer(&kafka.ConfigMap{
        "bootstrap.servers": "localhost:9092", // 根据实际情况修改
    })
    if err!= nil {
        return err
    }
    defer p.Close()

    deliveryChan := make(chan kafka.Event)
    err = p.Produce(&kafka.Message{
        TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
        Value:          data,
    }, deliveryChan)
    if err!= nil {
        close(deliveryChan)
        return err
    }

    e := <-deliveryChan
    m := e.(*kafka.Message)

    if m.TopicPartition.Error!= nil {
        close(deliveryChan)
        return m.TopicPartition.Error
    }

    close(deliveryChan)
    return nil
}

// 中间层服务的HTTP处理函数
func searchHandler(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query().Get("q")
    if query == "" {
        http.Error(w, "缺少查询参数q", http.StatusBadRequest)
        return
    }

    result, err := searchExternalEngine(query)
    if err!= nil {
        http.Error(w, "搜索失败", http.StatusInternalServerError)
        return
    }

    err = sendToKafka("your_topic", result)
    if err!= nil {
        http.Error(w, "发送到Kafka失败", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    w.Write([]byte("数据已成功发送到Kafka"))
}

func main() {
    err := installDependency()
    if err!= nil {
        fmt.Println("安装依赖失败:", err)
        return
    }

    http.HandleFunc("/search", searchHandler)
    fmt.Println("中间层服务正在监听 :8080")
    err = http.ListenAndServe(":8080", nil)
    if err!= nil {
        fmt.Println("服务启动失败:", err)
    }
}
