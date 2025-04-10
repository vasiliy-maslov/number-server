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

type Config struct {
	Port    string `json:"port"`
	LogFile string `json:"log_file"`
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
