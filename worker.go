package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"strings"
	"time"
)

type Worker interface {
	ProcessNumber(num int)
	GetStats() Stats
	Reset()
	GetLogs() string
	Ping() error
}

type PostgresWorker struct {
	db *sql.DB
}

func NewPostgresWorker(cfg DBConfig) (*PostgresWorker, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	var db *sql.DB
	var err error
	maxAttempts := 10
	for i := 0; i < maxAttempts; i++ {
		db, err = sql.Open("postgres", connStr)
		if err != nil {
			fmt.Printf("Попытка %d: не удалось открыть БД: %v\n", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		err = db.Ping()
		if err == nil {
			fmt.Printf("Успешное подключение к БД после %d попыток\n", i+1)
			return &PostgresWorker{db: db}, nil
		}
		fmt.Printf("Попытка %d: не удалось пинговать БД: %v\n", i+1, err)
		db.Close()
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("failed to connect to db after %d attempts: %v", maxAttempts, err)
}

func (pw *PostgresWorker) ProcessNumber(num int) {
	isEven := num%2 == 0
	_, err := pw.db.Exec("INSERT INTO numbers (number, is_even) VALUES ($1, $2)", num, isEven)
	if err != nil {
		fmt.Printf("Failed to insert number: %v\n", err)
		return
	}
	if isEven {
		_, err = pw.db.Exec("UPDATE stats SET even_count = even_count + 1 WHERE worker_id = 1")
	} else {
		_, err = pw.db.Exec("UPDATE stats SET odd_count = odd_count + 1 WHERE worker_id = 1")
	}
	if err != nil {
		fmt.Printf("Failed to update stats: %v\n", err)
	}
}

func (pw *PostgresWorker) GetStats() Stats {
	var total Stats
	row := pw.db.QueryRow("SELECT SUM(even_count), SUM(odd_count) FROM stats")
	if err := row.Scan(&total.Even, &total.Odd); err != nil {
		fmt.Printf("Failed to get stats: %v\n", err)
	}
	return total
}

func (pw *PostgresWorker) Reset() {
	_, err := pw.db.Exec("TRUNCATE numbers, stats; INSERT INTO stats (worker_id, even_count, odd_count) VALUES (1, 0, 0), (2, 0, 0)")
	if err != nil {
		fmt.Printf("Failed to reset: %v\n", err)
	}
}

func (pw *PostgresWorker) GetLogs() string {
	rows, err := pw.db.Query("SELECT number, is_even, processed_at FROM numbers ORDER BY processed_at")
	if err != nil {
		fmt.Printf("Failed to get logs: %v\n", err)
		return ""
	}
	defer rows.Close()
	var logs strings.Builder
	for rows.Next() {
		var num int
		var isEven bool
		var t time.Time
		if err := rows.Scan(&num, &isEven, &t); err != nil {
			fmt.Printf("Failed to scan log: %v\n", err)
			continue
		}
		kind := "ODD"
		if isEven {
			kind = "EVEN"
		}
		fmt.Fprintf(&logs, "[%s] [%s] Number: %d\n", t.Format("2006-01-02 15:04:05"), kind, num)
	}
	return logs.String()
}

func (pw *PostgresWorker) Ping() error {
	return pw.db.Ping()
}
