package main

import (
	"encoding/json"
	"fmt"
	"number-server/kafka"
	"number-server/models"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		fmt.Printf("Ошибка чтения конфига: %v\n", err)
		return
	}

	var cfg models.Config
	if err := json.Unmarshal(configFile, &cfg); err != nil {
		fmt.Printf("Ошибка разбора конфига: %v\n", err)
		return
	}

	wp, err := NewWorkerPool(cfg)
	if err != nil {
		fmt.Printf("Ошибка инициализации WorkerPool: %v\n", err)
		return
	}
	defer wp.Close()

	srv := NewServer(cfg, wp) // NewServer принимает models.Worker
	consumer := kafka.NewConsumer(srv.logger, wp, []string{"kafka:9092"}, "number-consumer")
	defer consumer.Close()

	if err := os.WriteFile(cfg.LogFile, []byte{}, 0644); err != nil {
		srv.logger.Printf("Ошибка очистки логов: %v", err)
		return
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			srv.logger.Fatalf("Ошибка запуска сервера: %v", err)
		}
	}()

	<-sigChan
	srv.logger.Println("Shutting down server...")
}
