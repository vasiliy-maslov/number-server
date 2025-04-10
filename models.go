package main

import (
	"log"
	"sync"
)

type Stats struct {
	Even int `json:"even"`
	Odd  int `json:"odd"`
}

type NumberRequest struct {
	Number int `json:"number"`
}

type NumberResponse struct {
	Status string `json:"status"`
}

type ResetResponse struct {
	Status string `json:"status"`
}

type HealthResponse struct {
	ServerStatus string `json:"status"`
	DBStatus     string `json:"db"`
}

type WorkerPool struct {
	numbersChan chan int
	statsChan   chan chan Stats
	resultsChan chan Stats
	resetChan   chan struct{}
	logMutex    *sync.Mutex
	logFile     string
}

type Server struct {
	port   string
	logger *log.Logger
	worker Worker
}

type DBConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

type Config struct {
	Port    string   `json:"port"`
	LogFile string   `json:"log_file"`
	DB      DBConfig `json:"db"`
}
