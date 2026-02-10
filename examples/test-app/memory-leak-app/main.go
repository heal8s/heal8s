package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"
)

var memoryLeakData [][]byte

func main() {
	leakRateMB := getEnvInt("LEAK_RATE_MB", 10)
	leakInterval := getEnvDuration("LEAK_INTERVAL", "5s")
	maxLeakMB := getEnvInt("MAX_LEAK_MB", 500)
	port := getEnv("PORT", "8080")

	log.Printf("Starting memory leak simulator...")
	log.Printf("Leak rate: %d MB every %s", leakRateMB, leakInterval)
	log.Printf("Max leak: %d MB", maxLeakMB)

	// Start memory leak goroutine
	go func() {
		ticker := time.NewTicker(leakInterval)
		defer ticker.Stop()

		totalLeakedMB := 0
		for range ticker.C {
			if totalLeakedMB >= maxLeakMB {
				log.Printf("Reached max leak size (%d MB), holding steady", maxLeakMB)
				continue
			}

			// Allocate memory
			chunk := make([]byte, leakRateMB*1024*1024)
			for i := range chunk {
				chunk[i] = byte(i % 256)
			}
			memoryLeakData = append(memoryLeakData, chunk)
			totalLeakedMB += leakRateMB

			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			log.Printf("Leaked %d MB total. Current alloc: %d MB, Sys: %d MB, NumGC: %d",
				totalLeakedMB,
				m.Alloc/1024/1024,
				m.Sys/1024/1024,
				m.NumGC)
		}
	}()

	// HTTP server for health checks and metrics
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Memory Leak Simulator\n")
		fmt.Fprintf(w, "Leaked chunks: %d\n", len(memoryLeakData))
		
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		fmt.Fprintf(w, "Alloc: %d MB\n", m.Alloc/1024/1024)
		fmt.Fprintf(w, "TotalAlloc: %d MB\n", m.TotalAlloc/1024/1024)
		fmt.Fprintf(w, "Sys: %d MB\n", m.Sys/1024/1024)
		fmt.Fprintf(w, "NumGC: %d\n", m.NumGC)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		fmt.Fprintf(w, "# HELP memory_leak_bytes Memory leaked in bytes\n")
		fmt.Fprintf(w, "# TYPE memory_leak_bytes gauge\n")
		fmt.Fprintf(w, "memory_leak_bytes %d\n", m.Alloc)
		
		fmt.Fprintf(w, "# HELP memory_leak_chunks Number of leaked memory chunks\n")
		fmt.Fprintf(w, "# TYPE memory_leak_chunks gauge\n")
		fmt.Fprintf(w, "memory_leak_chunks %d\n", len(memoryLeakData))
	})

	log.Printf("Starting HTTP server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue string) time.Duration {
	value := getEnv(key, defaultValue)
	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("Failed to parse duration %s: %v, using default", value, err)
		duration, _ = time.ParseDuration(defaultValue)
	}
	return duration
}
