package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

	wp := NewWorkerPool(cfg.LogFile)
	srv := NewServer(cfg, wp)

	if err := os.WriteFile(cfg.LogFile, []byte{}, 0644); err != nil {
		srv.logger.Printf("Ошибка очистки логов: %v", err)
		return
	}

	http.HandleFunc("/number", srv.handleNumber)
	http.HandleFunc("/stats", srv.handleStats)
	http.HandleFunc("/logs", srv.handleLogs)
	http.HandleFunc("/reset", srv.handleReset)

	srv.logger.Printf("Сервер запущен на %s", srv.port)
	if err := http.ListenAndServe(srv.port, nil); err != nil {
		srv.logger.Printf("Ошибка запуска сервера: %v", err)
	}
}
