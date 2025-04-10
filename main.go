package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		fmt.Printf("Ошибка чтения конфига: %v\n", err)
		return
	}

	var cfg Config
	if err := json.Unmarshal(configFile, &cfg); err != nil {
		fmt.Printf("Ошибка разбора конфига: %v\n", err)
		return
	}

	wp, err := NewPostgresWorker(cfg.DB)
	if err != nil {
		fmt.Printf("Ошибка подключения к БД: %v\n", err)
		return
	}
	defer func() {
		if err := wp.db.Close(); err != nil {
			fmt.Printf("Ошибка закрытия БД: %v\n", err)
		}
	}()

	srv := NewServer(cfg, wp)

	go func() {
		maxAttempts := 10
		for i := 0; i < maxAttempts; i++ {
			reader := kafka.NewReader(kafka.ReaderConfig{
				Brokers: []string{"kafka:9092"},
				Topic:   "numbers",
				GroupID: "number-consumer",
			})
			defer reader.Close()

			for {
				msg, err := reader.ReadMessage(context.Background())
				if err != nil {
					srv.logger.Printf("Попытка %d: ошибка чтения из Kafka: %v", i+1, err)
					time.Sleep(2 * time.Second)
					break
				}
				num, err := strconv.Atoi(string(msg.Value))
				if err != nil {
					srv.logger.Printf("Ошибка парсинга числа из Kafka: %v", err)
					continue
				}
				srv.worker.ProcessNumber(num)
				srv.logger.Printf("Обработано число из Kafka: %d", num)
			}
		}
	}()

	if err := os.WriteFile(cfg.LogFile, []byte{}, 0644); err != nil {
		srv.logger.Printf("Ошибка очистки логов: %v", err)
		return
	}

	http.HandleFunc("/number", srv.handleNumber)
	http.HandleFunc("/stats", srv.handleStats)
	http.HandleFunc("/logs", srv.handleLogs)
	http.HandleFunc("/reset", srv.handleReset)
	http.HandleFunc("/health", srv.handleHealth)

	srv.logger.Printf("Сервер запущен на %s", srv.port)
	if err := http.ListenAndServe(srv.port, nil); err != nil {
		srv.logger.Printf("Ошибка запуска сервера: %v", err)
	}
}
