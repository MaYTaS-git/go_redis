package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	Host         = "127.0.0.1"
	Port         = 6379
	RequestCount = 1000000 // 1 Million Requests per phase
	BatchStep    = 100000  // Log interval (every 100k requests)
)

func main() {
	addr := fmt.Sprintf("%s:%d", Host, Port)
	fmt.Printf("==============================================================================\n")
	fmt.Printf(" 🚀 GO-REDIS HEAVY LOAD BENCHMARK (1,000,000 REQUESTS)                        \n")
	fmt.Printf(" Target: %s | Total Requests: %d per phase                                    \n", addr, RequestCount)
	fmt.Printf("==============================================================================\n\n")

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Printf("[ERROR] Could not connect to Go-Redis server on %s: %v\n", addr, err)
		fmt.Println("Please make sure the server is running (e.g. ./go_redis.exe)")
		os.Exit(1)
	}
	defer conn.Close()

	reader := bufio.NewReaderSize(conn, 65536)

	// 1. Health Check
	fmt.Println("[1/3] Performing Health Check (PING)...")
	pingStart := time.Now()
	conn.Write([]byte("*1\r\n$4\r\nPING\r\n"))

	pingResp, err := readServerResponse(reader)
	pingDuration := time.Since(pingStart)

	if err != nil {
		fmt.Printf("[ERROR] PING failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[SUCCESS] Server responded with '%s' in %.3f ms\n\n", string(pingResp), float64(pingDuration.Microseconds())/1000.0)

	// 2. WRITE (SET 1,000,000 ops)
	fmt.Printf("[2/3] Executing Heavy Write Benchmark (%d SET requests)...\n", RequestCount)
	writeStartTotal := time.Now()
	var writeSumUs, writeMinUs, writeMaxUs int64
	writeMinUs = 1<<62 - 1
	stageStart := time.Now()

	for i := 0; i < RequestCount; i++ {
		start := time.Now()
		key := fmt.Sprintf("bkey_%d", i)
		val := fmt.Sprintf("v_%d", i)

		cmd := fmt.Sprintf("*3\r\n$3\r\nSET\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n", len(key), key, len(val), val)
		conn.Write([]byte(cmd))
		_, err := readServerResponse(reader)
		if err != nil {
			fmt.Printf("\n[ERROR] Write error at index %d: %v\n", i, err)
			break
		}

		elapsedUs := time.Since(start).Microseconds()
		writeSumUs += elapsedUs
		if elapsedUs < writeMinUs {
			writeMinUs = elapsedUs
		}
		if elapsedUs > writeMaxUs {
			writeMaxUs = elapsedUs
		}

		if (i+1)%BatchStep == 0 || i+1 == RequestCount {
			stageTime := time.Since(stageStart)
			pct := (float64(i+1) / float64(RequestCount)) * 100
			batchOpsSec := float64(BatchStep) / stageTime.Seconds()
			fmt.Printf("  └─ Writes Stage [%d - %d] (%d%%): took %.2f sec (%.2f ops/sec)\n",
				i+1-BatchStep+1, i+1, int(pct), stageTime.Seconds(), batchOpsSec)
			stageStart = time.Now()
		}
	}

	writeTotalDuration := time.Since(writeStartTotal)
	writeRps := float64(RequestCount) / writeTotalDuration.Seconds()
	writeAvgLat := (float64(writeSumUs) / float64(RequestCount)) / 1000.0
	writeMinLat := float64(writeMinUs) / 1000.0
	writeMaxLat := float64(writeMaxUs) / 1000.0

	// 3. READ (GET 1,000,000 ops)
	fmt.Printf("\n[3/3] Executing Heavy Read Benchmark (%d GET requests)...\n", RequestCount)
	readStartTotal := time.Now()
	var readSumUs, readMinUs, readMaxUs int64
	readMinUs = 1<<62 - 1
	stageStart = time.Now()

	for i := 0; i < RequestCount; i++ {
		start := time.Now()
		key := fmt.Sprintf("bkey_%d", i)

		cmd := fmt.Sprintf("*2\r\n$3\r\nGET\r\n$%d\r\n%s\r\n", len(key), key)
		conn.Write([]byte(cmd))
		_, err := readServerResponse(reader)
		if err != nil {
			fmt.Printf("\n[ERROR] Read error at index %d: %v\n", i, err)
			break
		}

		elapsedUs := time.Since(start).Microseconds()
		readSumUs += elapsedUs
		if elapsedUs < readMinUs {
			readMinUs = elapsedUs
		}
		if elapsedUs > readMaxUs {
			readMaxUs = elapsedUs
		}

		if (i+1)%BatchStep == 0 || i+1 == RequestCount {
			stageTime := time.Since(stageStart)
			pct := (float64(i+1) / float64(RequestCount)) * 100
			batchOpsSec := float64(BatchStep) / stageTime.Seconds()
			fmt.Printf("  └─ Reads Stage  [%d - %d] (%d%%): took %.2f sec (%.2f ops/sec)\n",
				i+1-BatchStep+1, i+1, int(pct), stageTime.Seconds(), batchOpsSec)
			stageStart = time.Now()
		}
	}

	readTotalDuration := time.Since(readStartTotal)
	readRps := float64(RequestCount) / readTotalDuration.Seconds()
	readAvgLat := (float64(readSumUs) / float64(RequestCount)) / 1000.0
	readMinLat := float64(readMinUs) / 1000.0
	readMaxLat := float64(readMaxUs) / 1000.0

	fmt.Println("\n==============================================================================")
	fmt.Println("🔥 1,000,000 REQUESTS BENCHMARK RESULTS")
	fmt.Println("==============================================================================")
	fmt.Printf("WRITE (SET 1,000,000 ops): %.2f sec total | %.2f ops/sec | Avg: %.3f ms (Min: %.3f ms, Max: %.3f ms)\n",
		writeTotalDuration.Seconds(), writeRps, writeAvgLat, writeMinLat, writeMaxLat)
	fmt.Printf("READ  (GET 1,000,000 ops): %.2f sec total | %.2f ops/sec | Avg: %.3f ms (Min: %.3f ms, Max: %.3f ms)\n",
		readTotalDuration.Seconds(), readRps, readAvgLat, readMinLat, readMaxLat)
	fmt.Println("==============================================================================")

	// Write performance.txt report
	reportPath := filepath.Join(".", "performance.txt")
	reportContent := fmt.Sprintf(`==============================================================================
GO-REDIS HEAVY LOAD BENCHMARK REPORT (1,000,000 REQUESTS)
Timestamp: %s
Target: %s
Requests per phase: %d
==============================================================================
STATUS: SERVER RESPONDING OK (PING latency: %.3f ms)

[SET OPERATIONS (WRITE - 1,000,000 REQS)]
Total Time:     %.2f sec (%.2f ms)
Throughput:     %.2f req/sec
Avg Latency:    %.3f ms
Min Latency:    %.3f ms
Max Latency:    %.3f ms

[GET OPERATIONS (READ - 1,000,000 REQS)]
Total Time:     %.2f sec (%.2f ms)
Throughput:     %.2f req/sec
Avg Latency:    %.3f ms
Min Latency:    %.3f ms
Max Latency:    %.3f ms
==============================================================================
`,
		time.Now().Format(time.RFC3339),
		addr,
		RequestCount,
		float64(pingDuration.Microseconds())/1000.0,
		writeTotalDuration.Seconds(), float64(writeTotalDuration.Microseconds())/1000.0,
		writeRps, writeAvgLat, writeMinLat, writeMaxLat,
		readTotalDuration.Seconds(), float64(readTotalDuration.Microseconds())/1000.0,
		readRps, readAvgLat, readMinLat, readMaxLat,
	)

	_ = os.WriteFile(reportPath, []byte(reportContent), 0644)
	fmt.Printf("\n[SUCCESS] 1 Million Requests Benchmark Report written to '%s'\n", reportPath)
}

// readServerResponse correctly parses all RESP2 server responses (+, -, :, $, *) from bufio.Reader.
func readServerResponse(rd *bufio.Reader) ([]byte, error) {
	line, err := rd.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 3 {
		return nil, fmt.Errorf("short response line")
	}

	switch line[0] {
	case '+', '-', ':':
		return bytes.TrimSuffix(line[1:], []byte("\r\n")), nil
	case '$':
		// Bulk String: $<len>\r\n<data>\r\n
		strLen, err := strconv.Atoi(string(bytes.TrimSuffix(line[1:], []byte("\r\n"))))
		if err != nil {
			return nil, err
		}
		if strLen == -1 {
			return nil, nil // Null bulk string
		}

		buf := make([]byte, strLen+2)
		if _, err := io.ReadFull(rd, buf); err != nil {
			return nil, err
		}
		return buf[:strLen], nil
	case '*':
		// Array
		numElems, err := strconv.Atoi(string(bytes.TrimSuffix(line[1:], []byte("\r\n"))))
		if err != nil {
			return nil, err
		}
		for i := 0; i < numElems; i++ {
			if _, err := readServerResponse(rd); err != nil {
				return nil, err
			}
		}
		return []byte("OK"), nil
	default:
		return bytes.TrimSuffix(line, []byte("\r\n")), nil
	}
}
