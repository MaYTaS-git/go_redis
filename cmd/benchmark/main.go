package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func main() {
	reqCountFlag := flag.Int("n", 1000000, "Total requests per phase")
	pipelineFlag := flag.Int("p", 64, "Pipeline batch size (1 = single ping-pong, 64 = high throughput pipelining)")
	hostFlag := flag.String("h", "127.0.0.1", "Target host")
	portFlag := flag.Int("port", 6379, "Target port")
	flag.Parse()

	requestCount := *reqCountFlag
	pipelineSize := *pipelineFlag
	if pipelineSize < 1 {
		pipelineSize = 1
	}

	addr := fmt.Sprintf("%s:%d", *hostFlag, *portFlag)
	fmt.Printf("==============================================================================\n")
	fmt.Printf(" 🚀 GO-REDIS HIGH-PERFORMANCE BENCHMARK SUITE                                \n")
	fmt.Printf(" Target: %s | Total Requests: %d | Pipeline Size: %d                          \n", addr, requestCount, pipelineSize)
	fmt.Printf("==============================================================================\n\n")

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Printf("[ERROR] Could not connect to Go-Redis server on %s: %v\n", addr, err)
		fmt.Println("Please make sure the server is running (e.g. ./go_redis.exe)")
		os.Exit(1)
	}
	defer conn.Close()

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = tcpConn.SetReadBuffer(512 * 1024)
		_ = tcpConn.SetWriteBuffer(512 * 1024)
	}

	reader := bufio.NewReaderSize(conn, 128*1024)
	writer := bufio.NewWriterSize(conn, 128*1024)

	// 1. Health Check
	fmt.Println("[1/3] Performing Health Check (PING)...")
	pingStart := time.Now()
	_, _ = writer.WriteString("*1\r\n$4\r\nPING\r\n")
	_ = writer.Flush()

	pingResp, err := readServerResponse(reader)
	pingDuration := time.Since(pingStart)

	if err != nil {
		fmt.Printf("[ERROR] PING failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[SUCCESS] Server responded with '%s' in %.3f ms\n\n", string(pingResp), float64(pingDuration.Microseconds())/1000.0)

	// 2. WRITE (SET 1,000,000 ops)
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
		for k := 0; k < batchSize; k++ {
			idx := i + k
			key := fmt.Sprintf("bkey_%d", idx)
			val := fmt.Sprintf("v_%d", idx)
			_, _ = fmt.Fprintf(writer, "*3\r\n$3\r\nSET\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n", len(key), key, len(val), val)
		}
		_ = writer.Flush()

		for k := 0; k < batchSize; k++ {
			_, err := readServerResponse(reader)
			if err != nil {
				fmt.Printf("\n[ERROR] Write response error at index %d: %v\n", i+k, err)
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

	// 3. READ (GET 1,000,000 ops)
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
		for k := 0; k < batchSize; k++ {
			idx := i + k
			key := fmt.Sprintf("bkey_%d", idx)
			_, _ = fmt.Fprintf(writer, "*2\r\n$3\r\nGET\r\n$%d\r\n%s\r\n", len(key), key)
		}
		_ = writer.Flush()

		for k := 0; k < batchSize; k++ {
			_, err := readServerResponse(reader)
			if err != nil {
				fmt.Printf("\n[ERROR] Read response error at index %d: %v\n", i+k, err)
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
	fmt.Printf("🔥 BENCHMARK RESULTS (%d REQUESTS, PIPELINE=%d)\n", requestCount, pipelineSize)
	fmt.Println("==============================================================================")
	fmt.Printf("WRITE (SET %d ops): %.2f sec total | %.2f ops/sec | Avg: %.3f ms (Min: %.3f ms, Max: %.3f ms)\n",
		requestCount, writeTotalDuration.Seconds(), writeRps, writeAvgLat, writeMinLat, writeMaxLat)
	fmt.Printf("READ  (GET %d ops): %.2f sec total | %.2f ops/sec | Avg: %.3f ms (Min: %.3f ms, Max: %.3f ms)\n",
		requestCount, readTotalDuration.Seconds(), readRps, readAvgLat, readMinLat, readMaxLat)
	fmt.Println("==============================================================================")

	// Write performance.txt report
	reportPath := filepath.Join(".", "performance.txt")
	reportContent := fmt.Sprintf(`==============================================================================
GO-REDIS HIGH-PERFORMANCE BENCHMARK REPORT
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
	fmt.Printf("\n[SUCCESS] Benchmark Report written to '%s'\n", reportPath)
}

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
		strLen, err := strconv.Atoi(string(bytes.TrimSuffix(line[1:], []byte("\r\n"))))
		if err != nil {
			return nil, err
		}
		if strLen == -1 {
			return nil, nil
		}

		buf := make([]byte, strLen+2)
		if _, err := io.ReadFull(rd, buf); err != nil {
			return nil, err
		}
		return buf[:strLen], nil
	case '*':
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
