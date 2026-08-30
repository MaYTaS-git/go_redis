package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	reqCountFlag := flag.Int("n", 1000000, "Total requests per phase (default: 1,000,000)")
	pipelineFlag := flag.Int("p", 64, "Pipeline batch size (1 = sequential ping-pong, 64 = high-throughput pipelining)")
	hostFlag := flag.String("h", "127.0.0.1", "Target host")
	portFlag := flag.Int("port", 6379, "Target port")
	flag.Parse()

	requestCount := *reqCountFlag
	pipelineSize := *pipelineFlag
	if pipelineSize < 1 {
		pipelineSize = 1
	}

	addr := fmt.Sprintf("%s:%d", *hostFlag, *portFlag)
	fmt.Println("==============================================================================")
	fmt.Println(" 🚀 EXTERNAL REDIS CLIENT BENCHMARK (`github.com/redis/go-redis/v9`)          ")
	fmt.Printf(" Target: %s | Total Requests: %d | Pipeline Size: %d\n", addr, requestCount, pipelineSize)
	fmt.Println("==============================================================================")
	fmt.Println()

	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Protocol: 2, // Standard RESP2
	})
	defer rdb.Close()

	// 1. Health Check
	fmt.Println("[1/3] Performing Health Check via go-redis/v9 (PING)...")
	pingStart := time.Now()
	pong, err := rdb.Ping(ctx).Result()
	pingDuration := time.Since(pingStart)

	if err != nil {
		fmt.Printf("[ERROR] Ping failed: %v\n", err)
		fmt.Println("Please make sure Go-Redis server is running (e.g. ./go_redis.exe)")
		os.Exit(1)
	}
	fmt.Printf("[SUCCESS] Server responded with '%s' in %.3f ms\n\n", pong, float64(pingDuration.Microseconds())/1000.0)

	// 2. Heavy Write Benchmark (SET 1,000,000 ops)
	fmt.Printf("[2/3] Executing Write Benchmark (%d SET requests, pipeline=%d)...\n", requestCount, pipelineSize)
	writeStartTotal := time.Now()
	var writeSumUs, writeMinUs, writeMaxUs int64
	writeMinUs = 1<<62 - 1
	stageStart := time.Now()
	batchStep := 100000

	for i := 0; i < requestCount; i += pipelineSize {
		batchSize := pipelineSize
		if i+batchSize > requestCount {
			batchSize = requestCount - i
		}

		start := time.Now()
		if pipelineSize > 1 {
			pipe := rdb.Pipeline()
			for k := 0; k < batchSize; k++ {
				idx := i + k
				key := fmt.Sprintf("ext_key_%d", idx)
				val := fmt.Sprintf("ext_val_%d", idx)
				pipe.Set(ctx, key, val, 0)
			}
			_, err := pipe.Exec(ctx)
			if err != nil {
				fmt.Printf("\n[ERROR] Pipeline Write error at index %d: %v\n", i, err)
				break
			}
		} else {
			key := fmt.Sprintf("ext_key_%d", i)
			val := fmt.Sprintf("ext_val_%d", i)
			err := rdb.Set(ctx, key, val, 0).Err()
			if err != nil {
				fmt.Printf("\n[ERROR] Write error at index %d: %v\n", i, err)
				break
			}
		}

		elapsedUs := time.Since(start).Microseconds()
		perReqUs := elapsedUs / int64(batchSize)
		writeSumUs += elapsedUs
		if perReqUs < writeMinUs {
			writeMinUs = perReqUs
		}
		if perReqUs > writeMaxUs {
			writeMaxUs = perReqUs
		}

		curr := i + batchSize
		if curr%batchStep == 0 || curr == requestCount {
			stageTime := time.Since(stageStart)
			pct := (float64(curr) / float64(requestCount)) * 100
			batchOpsSec := float64(batchStep) / stageTime.Seconds()
			fmt.Printf("  └─ Writes Stage [%d - %d] (%d%%): took %.2f sec (%.2f ops/sec)\n",
				curr-batchStep+1, curr, int(pct), stageTime.Seconds(), batchOpsSec)
			stageStart = time.Now()
		}
	}

	writeTotalDuration := time.Since(writeStartTotal)
	writeRps := float64(requestCount) / writeTotalDuration.Seconds()
	writeAvgLat := (float64(writeSumUs) / float64(requestCount)) / 1000.0
	writeMinLat := float64(writeMinUs) / 1000.0
	writeMaxLat := float64(writeMaxUs) / 1000.0

	// 3. Heavy Read Benchmark (GET 1,000,000 ops)
	fmt.Printf("\n[3/3] Executing Read Benchmark (%d GET requests, pipeline=%d)...\n", requestCount, pipelineSize)
	readStartTotal := time.Now()
	var readSumUs, readMinUs, readMaxUs int64
	readMinUs = 1<<62 - 1
	stageStart = time.Now()

	for i := 0; i < requestCount; i += pipelineSize {
		batchSize := pipelineSize
		if i+batchSize > requestCount {
			batchSize = requestCount - i
		}

		start := time.Now()
		if pipelineSize > 1 {
			pipe := rdb.Pipeline()
			for k := 0; k < batchSize; k++ {
				idx := i + k
				key := fmt.Sprintf("ext_key_%d", idx)
				pipe.Get(ctx, key)
			}
			_, err := pipe.Exec(ctx)
			if err != nil {
				fmt.Printf("\n[ERROR] Pipeline Read error at index %d: %v\n", i, err)
				break
			}
		} else {
			key := fmt.Sprintf("ext_key_%d", i)
			_, err := rdb.Get(ctx, key).Result()
			if err != nil {
				fmt.Printf("\n[ERROR] Read error at index %d: %v\n", i, err)
				break
			}
		}

		elapsedUs := time.Since(start).Microseconds()
		perReqUs := elapsedUs / int64(batchSize)
		readSumUs += elapsedUs
		if perReqUs < readMinUs {
			readMinUs = perReqUs
		}
		if perReqUs > readMaxUs {
			readMaxUs = perReqUs
		}

		curr := i + batchSize
		if curr%batchStep == 0 || curr == requestCount {
			stageTime := time.Since(stageStart)
			pct := (float64(curr) / float64(requestCount)) * 100
			batchOpsSec := float64(batchStep) / stageTime.Seconds()
			fmt.Printf("  └─ Reads Stage  [%d - %d] (%d%%): took %.2f sec (%.2f ops/sec)\n",
				curr-batchStep+1, curr, int(pct), stageTime.Seconds(), batchOpsSec)
			stageStart = time.Now()
		}
	}

	readTotalDuration := time.Since(readStartTotal)
	readRps := float64(requestCount) / readTotalDuration.Seconds()
	readAvgLat := (float64(readSumUs) / float64(requestCount)) / 1000.0
	readMinLat := float64(readMinUs) / 1000.0
	readMaxLat := float64(readMaxUs) / 1000.0

	fmt.Println("\n==============================================================================")
	fmt.Printf("🔥 GO-REDIS/V9 CLIENT RESULTS (%d REQUESTS, PIPELINE=%d)\n", requestCount, pipelineSize)
	fmt.Println("==============================================================================")
	fmt.Printf("WRITE (SET %d ops): %.2f sec total | %.2f ops/sec | Avg: %.3f ms (Min: %.3f ms, Max: %.3f ms)\n",
		requestCount, writeTotalDuration.Seconds(), writeRps, writeAvgLat, writeMinLat, writeMaxLat)
	fmt.Printf("READ  (GET %d ops): %.2f sec total | %.2f ops/sec | Avg: %.3f ms (Min: %.3f ms, Max: %.3f ms)\n",
		requestCount, readTotalDuration.Seconds(), readRps, readAvgLat, readMinLat, readMaxLat)
	fmt.Println("==============================================================================")

	// Write performance_external.txt report
	reportPath := filepath.Join(".", "performance_external.txt")
	reportContent := fmt.Sprintf(`==============================================================================
EXTERNAL CLIENT BENCHMARK REPORT (github.com/redis/go-redis/v9)
Timestamp: %s
Target: %s
Requests per phase: %d
Pipeline Batch Size: %d
==============================================================================
STATUS: SERVER RESPONDING OK (PING latency: %.3f ms)

[SET OPERATIONS (WRITE - %d REQS)]
Total Time:     %.2f sec (%.2f ms)
Throughput:     %.2f req/sec
Avg Latency:    %.3f ms
Min Latency:    %.3f ms
Max Latency:    %.3f ms

[GET OPERATIONS (READ - %d REQS)]
Total Time:     %.2f sec (%.2f ms)
Throughput:     %.2f req/sec
Avg Latency:    %.3f ms
Min Latency:    %.3f ms
Max Latency:    %.3f ms
==============================================================================
`,
		time.Now().Format(time.RFC3339),
		addr,
		requestCount,
		pipelineSize,
		float64(pingDuration.Microseconds())/1000.0,
		requestCount, writeTotalDuration.Seconds(), float64(writeTotalDuration.Microseconds())/1000.0,
		writeRps, writeAvgLat, writeMinLat, writeMaxLat,
		requestCount, readTotalDuration.Seconds(), float64(readTotalDuration.Microseconds())/1000.0,
		readRps, readAvgLat, readMinLat, readMaxLat,
	)

	_ = os.WriteFile(reportPath, []byte(reportContent), 0644)
	fmt.Printf("\n[SUCCESS] External Client Benchmark Report written to '%s'\n", reportPath)
}
