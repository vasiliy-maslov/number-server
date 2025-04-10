package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func NewServer(cfg Config, w Worker) *Server {
	return &Server{
		port:   cfg.Port,
		logger: log.New(os.Stdout, "server: ", log.LstdFlags),
		worker: w,
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

	s.worker.ProcessNumber(req.Number)

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
	if err := s.worker.Ping(); err != nil {
		s.logger.Printf("POST /health: ошибка подключения к БД: %v", err)
		healthResponse = HealthResponse{ServerStatus: "error", DBStatus: "disconnected"}
	}

	if err := json.NewEncoder(w).Encode(healthResponse); err != nil {
		s.logger.Printf("POST /health: ошибка кодирования: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
	s.logger.Printf("POST /health: подключение успешно")

}
