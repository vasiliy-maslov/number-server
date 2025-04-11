package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

type Worker interface {
	ProcessNumber(num int)
	GetStats() Stats
	Reset()
	GetLogs() string
	Ping() error
	Close() error
}

type numberEntry struct {
	num    int
	isEven bool
}

func NewWorkerPool(cfg Config) (*WorkerPool, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.User, cfg.DB.Password, cfg.DB.DBName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %v", err)
	}

	maxAttempts := 10
	for i := 0; i < maxAttempts; i++ {
		if err = db.Ping(); err == nil {
			fmt.Printf("Успешное подключение к БД после %d попыток\n", i+1)
			break
		}
		fmt.Printf("Попытка %d: не удалось пинговать БД: %v\n", i+1, err)
		if i == maxAttempts-1 {
			db.Close()
			return nil, fmt.Errorf("failed to connect to db after %d attempts: %v", maxAttempts, err)
		}
		time.Sleep(2 * time.Second)
	}

	flushInterval, err := time.ParseDuration(cfg.FlushInterval)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("invalid flush_interval: %v", err)
	}

	wp := &WorkerPool{
		db:          db,
		numbersChan: make(chan int, cfg.BufferSize),
		statsChan:   make(chan chan Stats),
		resetChan:   make(chan struct{}),
		logMutex:    &sync.Mutex{},
		logFile:     cfg.LogFile,
		stats:       Stats{Even: 0, Odd: 0},
		buffer:      make([]numberEntry, 0, cfg.BufferSize),
		closed:      make(chan struct{}),
	}

	wp.wg.Add(1)
	go wp.run(flushInterval)

	return wp, nil
}

func (wp *WorkerPool) run(flushInterval time.Duration) {
	defer wp.wg.Done()
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case num, ok := <-wp.numbersChan:
			if !ok {
				wp.flushBuffer()
				return
			}
			wp.processNumber(num)
			wp.bufferMutex.Lock()
			if len(wp.buffer) >= cap(wp.buffer) {
				wp.flushBuffer()
			}
			wp.bufferMutex.Unlock()
		case respChan := <-wp.statsChan:
			respChan <- wp.stats
		case <-wp.resetChan:
			wp.resetInternal()
		case <-ticker.C:
			wp.bufferMutex.Lock()
			if len(wp.buffer) > 0 {
				wp.flushBuffer()
			}
			wp.bufferMutex.Unlock()
		case <-wp.closed:
			wp.flushBuffer()
			return
		}
	}
}

func (wp *WorkerPool) processNumber(num int) {
	isEven := num%2 == 0
	wp.bufferMutex.Lock()
	wp.buffer = append(wp.buffer, numberEntry{num: num, isEven: isEven})
	if isEven {
		wp.stats.Even++
	} else {
		wp.stats.Odd++
	}
	wp.bufferMutex.Unlock()

	wp.logMutex.Lock()
	defer wp.logMutex.Unlock()
	logEntry := fmt.Sprintf("[%s] [%s] Number: %d\n", time.Now().Format("2006-01-02 15:04:05"), map[bool]string{true: "EVEN", false: "ODD"}[isEven], num)
	f, err := os.OpenFile(wp.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(logEntry); err != nil {
		fmt.Printf("Failed to write log: %v\n", err)
	}
}

func (wp *WorkerPool) flushBuffer() {
	wp.bufferMutex.Lock()
	defer wp.bufferMutex.Unlock()
	if len(wp.buffer) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := "INSERT INTO numbers (number, is_even) VALUES "
	values := make([]interface{}, 0, len(wp.buffer)*2)
	placeholders := make([]string, 0, len(wp.buffer))
	for i, entry := range wp.buffer {
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d)", 2*i+1, 2*i+2))
		values = append(values, entry.num, entry.isEven)
	}
	query += strings.Join(placeholders, ",")

	for attempt := 1; attempt <= 3; attempt++ {
		_, err := wp.db.ExecContext(ctx, query, values...)
		if err == nil {
			break
		}
		fmt.Printf("Attempt %d: failed to flush buffer to db: %v\n", attempt, err)
		if attempt == 3 {
			fmt.Printf("Failed to flush buffer after 3 attempts\n")
			return
		}
		time.Sleep(time.Second)
	}

	_, err := wp.db.ExecContext(ctx, "UPDATE stats SET even_count = $1, odd_count = $2 WHERE worker_id = 1", wp.stats.Even, wp.stats.Odd)
	if err != nil {
		fmt.Printf("Failed to sync stats to db: %v\n", err)
	}

	wp.buffer = wp.buffer[:0]
}

func (wp *WorkerPool) ProcessNumber(num int) {
	select {
	case wp.numbersChan <- num:
	default:
		fmt.Printf("Numbers channel full, dropping number: %d\n", num)
	}
}

func (wp *WorkerPool) GetStats() Stats {
	respChan := make(chan Stats)
	select {
	case wp.statsChan <- respChan:
		return <-respChan
	case <-time.After(2 * time.Second):
		fmt.Printf("Timeout waiting for stats\n")
		return wp.stats
	}
}

func (wp *WorkerPool) Reset() {
	select {
	case wp.resetChan <- struct{}{}:
	case <-time.After(2 * time.Second):
		fmt.Printf("Timeout waiting for reset\n")
	}
}

func (wp *WorkerPool) resetInternal() {
	wp.bufferMutex.Lock()
	defer wp.bufferMutex.Unlock()
	wp.logMutex.Lock()
	defer wp.logMutex.Unlock()

	wp.stats = Stats{Even: 0, Odd: 0}
	wp.buffer = wp.buffer[:0]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := wp.db.ExecContext(ctx, "TRUNCATE numbers, stats; INSERT INTO stats (worker_id, even_count, odd_count) VALUES (1, 0, 0), (2, 0, 0)")
	if err != nil {
		fmt.Printf("Failed to reset db: %v\n", err)
	}

	if err := os.WriteFile(wp.logFile, []byte{}, 0644); err != nil {
		fmt.Printf("Failed to reset log file: %v\n", err)
	}
}

func (wp *WorkerPool) GetLogs() string {
	wp.logMutex.Lock()
	defer wp.logMutex.Unlock()
	data, err := os.ReadFile(wp.logFile)
	if err != nil {
		fmt.Printf("Failed to read logs: %v\n", err)
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (wp *WorkerPool) Ping() error {
	return wp.db.Ping()
}

func (wp *WorkerPool) Close() error {
	close(wp.closed)
	wp.wg.Wait()
	return wp.db.Close()
}
