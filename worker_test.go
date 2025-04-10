package main

import (
	_ "database/sql"
	"testing"
)

func TestProcessNumber(t *testing.T) {
	cfg := DBConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "123456",
		DBName:   "number_server",
	}
	pw, err := NewPostgresWorker(cfg)
	if err != nil {
		t.Fatalf("Failed to create PostgresWorker: %v", err)
	}
	defer pw.db.Close()

	pw.ProcessNumber(42)
	var count int
	err = pw.db.QueryRow("SELECT COUNT(*) FROM numbers WHERE number = 42").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query numbers: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 record, got %d", count)
	}
}
