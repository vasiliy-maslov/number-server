package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type Worker interface {
	ProcessNumber(num int)
	GetStats() Stats
	Reset()
	GetLogs() string
}

func NewWorkerPool(logFile string) *WorkerPool {
	wp := &WorkerPool{
		numbersChan: make(chan int, 10),
		statsChan:   make(chan chan Stats),
		resultsChan: make(chan Stats),
		resetChan:   make(chan struct{}),
		logMutex:    &sync.Mutex{},
		logFile:     logFile,
	}

	go wp.worker()
	go wp.worker()

	return wp
}

func (wp *WorkerPool) worker() {
	logFile, err := os.OpenFile(wp.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Ошибка при открытии лога: ", err)
		return
	}
	defer logFile.Close()

	evenCount, oddCount := 0, 0

	for {
		select {
		case num := <-wp.numbersChan:
			wp.logMutex.Lock()
			logTime := time.Now().Format("2006-01-02 15:04:05")
			if num%2 == 0 {
				evenCount++
				_, err := fmt.Fprintf(logFile, "[%s] [EVEN] Number: %d, Total: %d\n", logTime, num, evenCount)
				if err != nil {
					fmt.Println("Ошибка при записи в лог: ", err)
					wp.logMutex.Unlock()
					return
				}
			} else {
				oddCount++
				_, err := fmt.Fprintf(logFile, "[%s] [ODD] Number: %d, Total: %d\n", logTime, num, oddCount)
				if err != nil {
					fmt.Println("Ошибка при записи в лог: ", err)
					wp.logMutex.Unlock()
					return
				}
			}
			wp.logMutex.Unlock()
		case replyChan := <-wp.statsChan:
			replyChan <- Stats{Even: evenCount, Odd: oddCount}
		case <-wp.resetChan:
			evenCount, oddCount = 0, 0
		}
	}
}

func (wp *WorkerPool) ProcessNumber(num int) {
	wp.numbersChan <- num
}

func (wp *WorkerPool) GetStats() Stats {
	respChan := make(chan Stats)
	wp.statsChan <- respChan
	wp.statsChan <- respChan
	totalStats := Stats{}

	for range 2 {
		stats := <-respChan
		totalStats.Even += stats.Even
		totalStats.Odd += stats.Odd
	}

	return totalStats
}

func (wp *WorkerPool) Reset() {
	wp.resetChan <- struct{}{}
	wp.resetChan <- struct{}{}
	wp.logMutex.Lock()
	defer wp.logMutex.Unlock()
	if err := os.WriteFile(wp.logFile, []byte{}, 0644); err != nil {
		fmt.Printf("Ошибка очистки лога: %v\n", err)
	}
}

func (wp *WorkerPool) GetLogs() string {
	wp.logMutex.Lock()
	defer wp.logMutex.Unlock()
	data, err := os.ReadFile(wp.logFile)
	if err != nil {
		// Логируем ошибку через fmt или другой способ, так как у WorkerPool нет logger'а
		fmt.Printf("Ошибка чтения логов: %v\n", err)
		return ""
	}
	return string(data)
}
