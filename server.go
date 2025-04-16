package main

import (
	"log"
	"net/http"
	"number-server/handlers"
	"number-server/kafka"
	"number-server/models"
	"os"
)

type APIServer struct {
	port   string
	logger *log.Logger
	h      *handlers.Handler
}

func NewServer(cfg models.Config, w models.Worker) *APIServer {
	logger := log.New(os.Stdout, "server: ", log.LstdFlags)
	producer := kafka.NewProducer([]string{"kafka:9092"})
	h := handlers.NewHandler(logger, w, producer)
	return &APIServer{
		port:   cfg.Port,
		logger: logger,
		h:      h,
	}
}

func (s *APIServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/number", s.h.Number)
	mux.HandleFunc("/stats", s.h.Stats)
	mux.HandleFunc("/logs", s.h.Logs)
	mux.HandleFunc("/reset", s.h.Reset)
	mux.HandleFunc("/health", s.h.Health)
	return mux
}

func (s *APIServer) Start() error {
	s.logger.Printf("Сервер запускается на :%s", s.port)
	return http.ListenAndServe(":"+s.port, s.Handler())
}
