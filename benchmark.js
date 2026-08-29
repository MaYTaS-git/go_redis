/**
 * Go-Redis Heavy Load Benchmark (1,000,000 Requests)
 * Native TCP RESP Client in Node.js with Stage Timing
 */

const net = require('net');
const fs = require('fs');
const path = require('path');

const HOST = '127.0.0.1';
const PORT = 6379;
const REQUEST_COUNT = 1000000; // 1 Million Requests per phase
const BATCH_STEP = 100000;     // Log interval (every 100k requests)
const PERFORMANCE_FILE = path.join(__dirname, 'performance.txt');

function encodeRespCommand(args) {
    let buf = `*${args.length}\r\n`;
    for (const arg of args) {
        const str = String(arg);
        buf += `$${Buffer.byteLength(str)}\r\n${str}\r\n`;
    }
    return buf;
}

class RedisClient {
    constructor(host, port) {
        this.host = host;
        this.port = port;
        this.client = null;
        this.buffer = '';
        this.pendingResolves = [];
    }

    connect() {
        return new Promise((resolve, reject) => {
            this.client = net.createConnection({ host: this.host, port: this.port }, () => {
                resolve();
            });

            this.client.on('data', (data) => {
                this.buffer += data.toString('utf8');
                this._processBuffer();
            });

            this.client.on('error', (err) => {
                if (this.pendingResolves.length > 0) {
                    const req = this.pendingResolves.shift();
                    req.reject(err);
                } else {
                    reject(err);
                }
            });
        });
    }

    _processBuffer() {
        while (this.pendingResolves.length > 0 && this.buffer.length > 0) {
            const index = this.buffer.indexOf('\r\n');
            if (index === -1) break;

            const line = this.buffer.substring(0, index);
            const type = line[0];

            if (type === '+' || type === '-' || type === ':') {
                this.buffer = this.buffer.substring(index + 2);
                const req = this.pendingResolves.shift();
                req.resolve(line.substring(1));
            } else if (type === '$') {
                const len = parseInt(line.substring(1), 10);
                if (len === -1) {
                    this.buffer = this.buffer.substring(index + 2);
                    const req = this.pendingResolves.shift();
                    req.resolve(null);
                } else {
                    const totalNeeded = index + 2 + len + 2;
                    if (this.buffer.length < totalNeeded) break;

                    const val = this.buffer.substring(index + 2, index + 2 + len);
                    this.buffer = this.buffer.substring(totalNeeded);
                    const req = this.pendingResolves.shift();
                    req.resolve(val);
                }
            } else if (type === '*') {
                this.buffer = this.buffer.substring(index + 2);
                const req = this.pendingResolves.shift();
                req.resolve(line);
            } else {
                // Skips unrecognized characters if buffer misaligned
                this.buffer = this.buffer.substring(index + 2);
            }
        }
    }

    sendCommand(args) {
        return new Promise((resolve, reject) => {
            this.pendingResolves.push({ resolve, reject });
            this.client.write(encodeRespCommand(args));
        });
    }

    close() {
        if (this.client) {
            this.client.end();
        }
    }
}

async function runBenchmark() {
    console.log(`==============================================================================`);
    console.log(` 🚀 GO-REDIS HEAVY LOAD BENCHMARK (1,000,000 REQUESTS)                        `);
    console.log(` Target: ${HOST}:${PORT} | Total Requests: ${REQUEST_COUNT} per phase`);
    console.log(`==============================================================================\n`);

    const client = new RedisClient(HOST, PORT);

    try {
        console.log(`[1/3] Connecting to Go-Redis server...`);
        await client.connect();
        console.log(`[SUCCESS] Connected to ${HOST}:${PORT}`);

        // Health Check
        console.log(`\n[2/3] Performing Health Check (PING)...`);
        const pingStart = process.hrtime.bigint();
        const pong = await client.sendCommand(['PING']);
        const pingTimeMs = Number(process.hrtime.bigint() - pingStart) / 1e6;

        console.log(`[SUCCESS] Server responded with '${pong}' in ${pingTimeMs.toFixed(3)} ms`);

        // Heavy Write Benchmark
        console.log(`\n[3/3] Executing Heavy Write Benchmark (${REQUEST_COUNT} SET requests)...`);
        let writeSumMs = 0;
        let writeMinMs = Infinity;
        let writeMaxMs = 0;

        const writeStartTotal = process.hrtime.bigint();
        let stageStart = process.hrtime.bigint();

        for (let i = 0; i < REQUEST_COUNT; i++) {
            const start = process.hrtime.bigint();
            await client.sendCommand(['SET', `bkey_${i}`, `v_${i}`]);
            const elapsedMs = Number(process.hrtime.bigint() - start) / 1e6;

            writeSumMs += elapsedMs;
            if (elapsedMs < writeMinMs) writeMinMs = elapsedMs;
            if (elapsedMs > writeMaxMs) writeMaxMs = elapsedMs;

            if ((i + 1) % BATCH_STEP === 0 || i + 1 === REQUEST_COUNT) {
                const stageSec = Number(process.hrtime.bigint() - stageStart) / 1e9;
                const pct = Math.round(((i + 1) / REQUEST_COUNT) * 100);
                const stageOps = (BATCH_STEP / stageSec).toFixed(2);
                console.log(`  └─ Writes Stage [${i + 1 - BATCH_STEP + 1} - ${i + 1}] (${pct}%): took ${stageSec.toFixed(2)} sec (${stageOps} ops/sec)`);
                stageStart = process.hrtime.bigint();
            }
        }
        const writeTotalSec = Number(process.hrtime.bigint() - writeStartTotal) / 1e9;
        const writeRps = (REQUEST_COUNT / writeTotalSec).toFixed(2);
        const writeAvgLat = (writeSumMs / REQUEST_COUNT).toFixed(3);

        // Heavy Read Benchmark
        console.log(`\nExecuting Heavy Read Benchmark (${REQUEST_COUNT} GET requests)...`);
        let readSumMs = 0;
        let readMinMs = Infinity;
        let readMaxMs = 0;

        const readStartTotal = process.hrtime.bigint();
        stageStart = process.hrtime.bigint();

        for (let i = 0; i < REQUEST_COUNT; i++) {
            const start = process.hrtime.bigint();
            await client.sendCommand(['GET', `bkey_${i}`]);
            const elapsedMs = Number(process.hrtime.bigint() - start) / 1e6;

            readSumMs += elapsedMs;
            if (elapsedMs < readMinMs) readMinMs = elapsedMs;
            if (elapsedMs > readMaxMs) readMaxMs = elapsedMs;

            if ((i + 1) % BATCH_STEP === 0 || i + 1 === REQUEST_COUNT) {
                const stageSec = Number(process.hrtime.bigint() - stageStart) / 1e9;
                const pct = Math.round(((i + 1) / REQUEST_COUNT) * 100);
                const stageOps = (BATCH_STEP / stageSec).toFixed(2);
                console.log(`  └─ Reads Stage  [${i + 1 - BATCH_STEP + 1} - ${i + 1}] (${pct}%): took ${stageSec.toFixed(2)} sec (${stageOps} ops/sec)`);
                stageStart = process.hrtime.bigint();
            }
        }
        const readTotalSec = Number(process.hrtime.bigint() - readStartTotal) / 1e9;
        const readRps = (REQUEST_COUNT / readTotalSec).toFixed(2);
        const readAvgLat = (readSumMs / REQUEST_COUNT).toFixed(3);

        console.log(`\n==============================================================================`);
        console.log(`🔥 1,000,000 REQUESTS BENCHMARK RESULTS`);
        console.log(`==============================================================================`);
        console.log(`WRITE (SET 1,000,000 ops): ${writeTotalSec.toFixed(2)} sec total | ${writeRps} ops/sec | Avg: ${writeAvgLat} ms (Min: ${writeMinMs.toFixed(3)} ms, Max: ${writeMaxMs.toFixed(3)} ms)`);
        console.log(`READ  (GET 1,000,000 ops): ${readTotalSec.toFixed(2)} sec total | ${readRps} ops/sec | Avg: ${readAvgLat} ms (Min: ${readMinMs.toFixed(3)} ms, Max: ${readMaxMs.toFixed(3)} ms)`);
        console.log(`==============================================================================`);

        // Format performance log report
        const logContent = `==============================================================================
GO-REDIS HEAVY LOAD BENCHMARK REPORT (1,000,000 REQUESTS)
Timestamp: ${new Date().toISOString()}
Target: ${HOST}:${PORT}
Requests per phase: ${REQUEST_COUNT}
==============================================================================
STATUS: SERVER RESPONDING OK (PING latency: ${pingTimeMs.toFixed(3)} ms)

[SET OPERATIONS (WRITE - 1,000,000 REQS)]
Total Time:     ${writeTotalSec.toFixed(2)} sec (${(writeTotalSec * 1000).toFixed(2)} ms)
Throughput:     ${writeRps} req/sec
Avg Latency:    ${writeAvgLat} ms
Min Latency:    ${writeMinMs.toFixed(3)} ms
Max Latency:    ${writeMaxMs.toFixed(3)} ms

[GET OPERATIONS (READ - 1,000,000 REQS)]
Total Time:     ${readTotalSec.toFixed(2)} sec (${(readTotalSec * 1000).toFixed(2)} ms)
Throughput:     ${readRps} req/sec
Avg Latency:    ${readAvgLat} ms
Min Latency:    ${readMinMs.toFixed(3)} ms
Max Latency:    ${readMaxLat.toFixed(3)} ms
==============================================================================
\n`;

        fs.writeFileSync(PERFORMANCE_FILE, logContent);
        console.log(`\n[SUCCESS] 1 Million Requests Benchmark Report written to '${PERFORMANCE_FILE}'`);

        client.close();
    } catch (err) {
        console.error(`\n[ERROR] Server communication error:`, err.message);
        process.exit(1);
    }
}

runBenchmark();
