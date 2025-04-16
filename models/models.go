package models

import (
	"database/sql"
	"sync"
)

type NumberEntry struct {
	Num    int  `json:"num"`
	IsEven bool `json:"is_even"`
}

type Stats struct {
	Even int `json:"even"`
	Odd  int `json:"odd"`
}

type Worker interface {
	ProcessNumber(num int)
	GetStats() Stats
	Reset()
	GetLogs() string
	Ping() error
	Close() error
	LogRequest(num int) error
}

type WorkerPool struct {
	DB          *sql.DB
	NumbersChan chan int
	StatsChan   chan chan Stats
	ResetChan   chan struct{}
	LogMutex    *sync.Mutex
	LogFile     string
	StatsMutex  sync.Mutex
	Stats       Stats
	BufferMutex sync.Mutex
	Buffer      []NumberEntry
	WG          sync.WaitGroup
	Closed      chan struct{}
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
