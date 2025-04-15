package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"./models"
	"number-server/kafka"
)

type Handler struct {
	logger      *log.Logger
	worker      models.Worker
	kafkaWriter *kafka.Producer
}

func NewHandler(logger *log.Logger, worker models.Worker, kafkaWriter *kafka.Producer) *Handler {
	return &Handler{logger: logger, worker: worker, kafkaWriter: kafkaWriter}
}

func (h *Handler) Number(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.logger.Printf("POST /number: неверный метод: %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req models.NumberRequest
	h.logger.Printf("POST /number: декодирование числа")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Printf("POST /number: ошибка декодирования: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err := h.kafkaWriter.Send(r.Context(), "numbers", fmt.Sprintf("%d", req.Number))
	if err != nil {
		h.logger.Printf("POST /number: ошибка отправки в Kafka: %v", err)
		http.Error(w, "Error sending to Kafka", http.StatusInternalServerError)
		return
	}
	h.logger.Printf("POST /number: число %d успешно отправлено в Kafka", req.Number)

	resp := models.NumberResponse{Status: "odd"}
	if req.Number%2 == 0 {
		resp = models.NumberResponse{Status: "even"}
	}
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Printf("POST /number: ошибка кодирования: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
	h.logger.Printf("POST /number: обработано число %d, ответ %s", req.Number, resp.Status)
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.logger.Printf("GET /stats: неверный метод: %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	h.logger.Printf("GET /stats: сбор статистики")
	totalStats := h.worker.GetStats()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(totalStats); err != nil {
		h.logger.Printf("GET /stats: ошибка кодирования: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
	h.logger.Printf("GET /stats: отправлена статистика even=%d, odd=%d", totalStats.Even, totalStats.Odd)
}

func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.logger.Printf("GET /logs: неверный метод: %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	h.logger.Printf("GET /logs: чтение логов")
	logs := h.worker.GetLogs()

	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(logs)); err != nil {
		h.logger.Printf("GET /logs: ошибка отправки: %v", err)
		http.Error(w, "Error writing response", http.StatusInternalServerError)
		return
	}
	h.logger.Printf("GET /logs: логи отправлены")
}

func (h *Handler) Reset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.logger.Printf("POST /reset: неверный метод: %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	h.logger.Printf("POST /reset: сброс логов")
	h.worker.Reset()

	w.Header().Set("Content-Type", "application/json")
	resetResponse := models.ResetResponse{Status: "reset completed"}
	if err := json.NewEncoder(w).Encode(resetResponse); err != nil {
		h.logger.Printf("POST /reset: ошибка кодирования: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
	h.logger.Printf("POST /reset: сброс выполнен")
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.logger.Printf("GET /health: неверный метод: %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	healthResponse := models.HealthResponse{ServerStatus: "ok", DBStatus: "connected"}
	w.Header().Set("Content-Type", "application/json")
	if err := h.worker.Ping(); err != nil {
		h.logger.Printf("GET /health: ошибка подключения к БД: %v", err)
		healthResponse = models.HealthResponse{ServerStatus: "error", DBStatus: "disconnected"}
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	if err := json.NewEncoder(w).Encode(healthResponse); err != nil {
		h.logger.Printf("GET /health: ошибка кодирования: %v", err)
		return
	}
	h.logger.Printf("GET /health: подключение успешно")
}
