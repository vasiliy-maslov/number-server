package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
)

func NewServer(cfg Config, w Worker) *Server {
	return &Server{
		port:   cfg.Port,
		logger: log.New(os.Stdout, "server: ", log.LstdFlags),
		worker: w,
		kafkaWriter: &kafka.Writer{
			Addr:     kafka.TCP("kafka:9092"),
			Topic:    "numbers",
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (s *Server) handleNumber(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req NumberRequest
	s.logger.Printf("POST /number: декодирование числа")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Printf("POST /number: ошибка декодирования: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	msg := kafka.Message{
		Value: []byte(fmt.Sprintf("%d", req.Number)),
	}
	maxAttempts := 5
	for i := 0; i < maxAttempts; i++ {
		err := s.kafkaWriter.WriteMessages(context.Background(), msg)
		if err == nil {
			break
		}
		s.logger.Printf("Попытка %d: ошибка отправки в Kafka: %v", i+1, err)
		if i == maxAttempts-1 {
			http.Error(w, "Error sending to Kafka", http.StatusInternalServerError)
			return
		}
		time.Sleep(2 * time.Second)
	}

	resp := NumberResponse{Status: "odd"}
	if req.Number%2 == 0 {
		resp = NumberResponse{Status: "even"}
	}
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Printf("POST /number: ошибка кодирования: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
	s.logger.Printf("POST /number: обработано число %d, ответ %s", req.Number, resp.Status)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	s.logger.Printf("GET /stats: сбор статистики")
	totalStats := s.worker.GetStats()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(totalStats); err != nil {
		s.logger.Printf("GET /stats: ошибка кодирования: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
	s.logger.Printf("GET /stats: отправлена статистика even=%d, odd=%d", totalStats.Even, totalStats.Odd)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	s.logger.Printf("GET /logs: чтение логов")
	logs := s.worker.GetLogs()

	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(logs)); err != nil {
		s.logger.Printf("GET /logs: ошибка отправки: %v", err)
		http.Error(w, "Error writing response", http.StatusInternalServerError)
		return
	}
	s.logger.Printf("GET /logs: логи отправлены")
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	s.logger.Printf("POST /reset: сброс логов")
	s.worker.Reset()

	w.Header().Set("Content-Type", "application/json")
	resetResponse := ResetResponse{Status: "reset completed"}
	if err := json.NewEncoder(w).Encode(resetResponse); err != nil {
		s.logger.Printf("POST /reset: ошибка кодирования: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
	s.logger.Printf("POST /reset: сброс выполнен")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	healthResponse := HealthResponse{ServerStatus: "ok", DBStatus: "connected"}
	w.Header().Set("Content-Type", "application/json")
	if err := s.worker.Ping(); err != nil {
		s.logger.Printf("GET /health: ошибка подключения к БД: %v", err)
		healthResponse = HealthResponse{ServerStatus: "error", DBStatus: "disconnected"}
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	if err := json.NewEncoder(w).Encode(healthResponse); err != nil {
		s.logger.Printf("GET /health: ошибка кодирования: %v", err)
		return
	}
	s.logger.Printf("GET /health: подключение успешно")
}
