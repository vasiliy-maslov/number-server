package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"number-server/models"
	"os"
	"strings"
	"sync"
	"time"
)

type workerPool struct {
	inner *models.WorkerPool
}

func NewWorkerPool(cfg models.Config) (*workerPool, error) {
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

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS requests (id SERIAL PRIMARY KEY, number INT, received_at TIMESTAMP)")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create requests table: %v", err)
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS numbers (id SERIAL PRIMARY KEY, number INT, is_even BOOLEAN);")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create numbers table: %v", err)
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS stats (worker_id INT PRIMARY KEY, even_count INT, odd_count INT);")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create stats table: %v", err)
	}
	_, err = db.Exec("INSERT INTO stats (worker_id, even_count, odd_count) VALUES (1, 0, 0) ON CONFLICT DO NOTHING;")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to insert to stats table: %v", err)
	}

	flushInterval, err := time.ParseDuration(cfg.FlushInterval)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("invalid flush_interval: %v", err)
	}

	inner := &models.WorkerPool{
		DB:          db,
		NumbersChan: make(chan int, cfg.BufferSize),
		StatsChan:   make(chan chan models.Stats),
		ResetChan:   make(chan struct{}),
		LogMutex:    &sync.Mutex{},
		LogFile:     cfg.LogFile,
		Stats:       models.Stats{Even: 0, Odd: 0},
		Buffer:      make([]models.NumberEntry, 0, cfg.BufferSize),
		BufferMutex: sync.Mutex{},
		WG:          sync.WaitGroup{},
		Closed:      make(chan struct{}),
	}

	wp := &workerPool{inner: inner}
	wp.inner.WG.Add(1)
	go wp.run(flushInterval)

	return wp, nil
}

func (wp *workerPool) run(flushInterval time.Duration) {
	defer wp.inner.WG.Done()
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case num, ok := <-wp.inner.NumbersChan:
			if !ok {
				fmt.Printf("NumbersChan closed, flushing buffer\n")
				wp.flushBuffer()
				return
			}
			fmt.Printf("Received number %d from NumbersChan\n", num)
			wp.processNumber(num)
			wp.inner.BufferMutex.Lock()
			if len(wp.inner.Buffer) > 0 {
				fmt.Printf("Flushing buffer after processing %d\n", num)
				go wp.flushBuffer() // Горутина
			}
			wp.inner.BufferMutex.Unlock()
		case <-wp.inner.ResetChan:
			wp.resetInternal()
		case <-ticker.C:
			wp.inner.BufferMutex.Lock()
			if len(wp.inner.Buffer) > 0 {
				fmt.Printf("Ticker fired, flushing buffer\n")
				go wp.flushBuffer() // Горутина
			}
			wp.inner.BufferMutex.Unlock()
		case <-wp.inner.Closed:
			wp.flushBuffer()
			return
		}
	}
}

func (wp *workerPool) LogRequest(num int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := wp.inner.DB.ExecContext(ctx, "INSERT INTO requests (number, received_at) VALUES ($1, NOW())", num)
	return err
}

func (wp *workerPool) processNumber(num int) {
	isEven := num%2 == 0
	wp.inner.StatsMutex.Lock()
	if isEven {
		wp.inner.Stats.Even++
	} else {
		wp.inner.Stats.Odd++
	}
	fmt.Printf("Processing %d, stats: even=%d, odd=%d\n", num, wp.inner.Stats.Even, wp.inner.Stats.Odd)
	wp.inner.StatsMutex.Unlock()

	wp.inner.BufferMutex.Lock()
	wp.inner.Buffer = append(wp.inner.Buffer, models.NumberEntry{Num: num, IsEven: isEven})
	wp.inner.BufferMutex.Unlock()

	wp.inner.LogMutex.Lock()
	defer wp.inner.LogMutex.Unlock()
	logEntry := fmt.Sprintf("[%s] [%s] Number: %d\n", time.Now().Format("2006-01-02 15:04:05"), map[bool]string{true: "EVEN", false: "ODD"}[isEven], num)
	f, err := os.OpenFile(wp.inner.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(logEntry); err != nil {
		fmt.Printf("Failed to write log: %v\n", err)
	}
}

func (wp *workerPool) flushBuffer() {
	wp.inner.BufferMutex.Lock()
	fmt.Printf("Starting flushBuffer, buffer size: %d\n", len(wp.inner.Buffer))
	if len(wp.inner.Buffer) == 0 {
		wp.inner.BufferMutex.Unlock()
		return
	}
	bufferCopy := make([]models.NumberEntry, len(wp.inner.Buffer))
	copy(bufferCopy, wp.inner.Buffer)
	wp.inner.Buffer = wp.inner.Buffer[:0]
	wp.inner.BufferMutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Printf("Preparing to write %d entries to numbers\n", len(bufferCopy))
	query := "INSERT INTO numbers (number, is_even) VALUES "
	values := make([]interface{}, 0, len(bufferCopy)*2)
	placeholders := make([]string, 0, len(bufferCopy))
	for i, entry := range bufferCopy {
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d)", 2*i+1, 2*i+2))
		values = append(values, entry.Num, entry.IsEven)
	}
	if len(placeholders) == 0 {
		fmt.Printf("No placeholders generated, skipping DB write\n")
		return
	}
	query += strings.Join(placeholders, ",")
	fmt.Printf("Executing query: %s with %d values\n", query, len(values))

	for attempt := 1; attempt <= 3; attempt++ {
		result, err := wp.inner.DB.ExecContext(ctx, query, values...)
		if err != nil {
			fmt.Printf("Attempt %d: failed to flush buffer to numbers: %v\n", attempt, err)
			if attempt == 3 {
				fmt.Printf("Failed to flush buffer to numbers after 3 attempts\n")
				return
			}
			time.Sleep(time.Second)
			continue
		}
		rows, _ := result.RowsAffected()
		fmt.Printf("Successfully flushed %d entries to numbers, rows affected: %d\n", len(bufferCopy), rows)
		break
	}

	wp.inner.StatsMutex.Lock()
	fmt.Printf("Updating stats: even=%d, odd=%d\n", wp.inner.Stats.Even, wp.inner.Stats.Odd)
	wp.inner.StatsMutex.Unlock()
	_, err := wp.inner.DB.ExecContext(ctx, "UPDATE stats SET even_count = $1, odd_count = $2 WHERE worker_id = 1", wp.inner.Stats.Even, wp.inner.Stats.Odd)
	if err != nil {
		fmt.Printf("Failed to sync stats to db: %v\n", err)
	} else {
		fmt.Printf("Successfully updated stats\n")
	}
}

func (wp *workerPool) ProcessNumber(num int) {
	fmt.Printf("Sending number %d to NumbersChan\n", num)
	select {
	case wp.inner.NumbersChan <- num:
		fmt.Printf("Sent number %d to NumbersChan\n", num)
	default:
		fmt.Printf("Numbers channel full, dropping number: %d\n", num)
	}
}

func (wp *workerPool) GetStats() models.Stats {
	wp.inner.StatsMutex.Lock()
	stats := wp.inner.Stats
	fmt.Printf("Getting stats: even=%d, odd=%d\n", stats.Even, stats.Odd)
	wp.inner.StatsMutex.Unlock()
	return stats
}

func (wp *workerPool) Reset() {
	select {
	case wp.inner.ResetChan <- struct{}{}:
	case <-time.After(2 * time.Second):
		fmt.Printf("Timeout waiting for reset\n")
	}
}

func (wp *workerPool) resetInternal() {
	wp.inner.BufferMutex.Lock()
	defer wp.inner.BufferMutex.Unlock()
	wp.inner.LogMutex.Lock()
	defer wp.inner.LogMutex.Unlock()

	wp.inner.Stats = models.Stats{Even: 0, Odd: 0}
	wp.inner.Buffer = wp.inner.Buffer[:0]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := wp.inner.DB.ExecContext(ctx, "TRUNCATE numbers, stats; INSERT INTO stats (worker_id, even_count, odd_count) VALUES (1, 0, 0), (2, 0, 0)")
	if err != nil {
		fmt.Printf("Failed to reset db: %v\n", err)
	}

	if err := os.WriteFile(wp.inner.LogFile, []byte{}, 0644); err != nil {
		fmt.Printf("Failed to reset log file: %v\n", err)
	}
}

func (wp *workerPool) GetLogs() string {
	wp.inner.LogMutex.Lock()
	defer wp.inner.LogMutex.Unlock()
	data, err := os.ReadFile(wp.inner.LogFile)
	if err != nil {
		fmt.Printf("Failed to read logs: %v\n", err)
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (wp *workerPool) Ping() error {
	return wp.inner.DB.Ping()
}

func (wp *workerPool) Close() error {
	close(wp.inner.Closed)
	wp.inner.WG.Wait()
	return wp.inner.DB.Close()
}
