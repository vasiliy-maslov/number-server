package main

import (
	"database/sql"
	"log"
	"sync"

	"github.com/segmentio/kafka-go"
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

type Worker interface {
	ProcessNumber(num int)
	GetStats() Stats
	Reset()
	GetLogs() string
	Ping() error
	Close() error
}

type WorkerPool struct {
	db          *sql.DB
	numbersChan chan int
	statsChan   chan chan Stats
	resetChan   chan struct{}
	logMutex    *sync.Mutex
	logFile     string
	stats       Stats
	buffer      []numberEntry
	bufferMutex sync.Mutex
	wg          sync.WaitGroup
	closed      chan struct{}
}

type Server struct {
	port        string
	logger      *log.Logger
	worker      Worker
	kafkaWriter *kafka.Writer
}

type DBConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

type Config struct {
	Port          string   `json:"port"`
	LogFile       string   `json:"log_file"`
	DB            DBConfig `json:"db"`
	BufferSize    int      `json:"buffer_size"`
	FlushInterval string   `json:"flush_interval"`
}
