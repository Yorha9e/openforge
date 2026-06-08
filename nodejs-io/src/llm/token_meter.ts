export interface TokenRecord {
  pipelineId: string;
  projectId: string;
  provider: string;
  model: string;
  inputTokens: number;
  outputTokens: number;
  timestamp: Date;
}

interface ProviderSummary {
  provider: string;
  inputTokens: bigint;
  outputTokens: bigint;
  requestCount: number;
}

export interface UsageSummary {
  totalInputTokens: bigint;
  totalOutputTokens: bigint;
  totalCost: number;
  byProvider: ProviderSummary[];
  recordCount: number;
}

/**
 * FlushTransport receives the batched TokenRecord array on every flush.
 * Default is a no-op (logs to stdout). The Node server wires a real transport
 * that calls LLMRouterService.RecordTokenUsage over ConnectRPC to persist
 * records into the Go coordinate layer's token_usage table.
 *
 * The transport is responsible for at-least-once delivery: if it throws,
 * TokenMeter re-enqueues the batch (bounded by 2 * maxBufferSize to prevent
 * unbounded growth on persistent transport failure).
 */
export type FlushTransport = (batch: TokenRecord[]) => Promise<void>;

const defaultTransport: FlushTransport = async (batch) => {
  console.log(JSON.stringify({ event: "token_batch", count: batch.length, records: batch }));
};

export class TokenMeter {
  private buffer: TokenRecord[] = [];
  private allRecords: TokenRecord[] = [];
  private readonly maxBufferSize = 500;
  private readonly maxRecords = 10_000;
  private flushIntervalMs = 5000;

  private cumulativeInput = 0n;
  private cumulativeOutput = 0n;
  private cumulativeCount = 0;

  constructor(private transport: FlushTransport = defaultTransport) {}

  record(record: TokenRecord): void {
    this.buffer.push(record);
    this.allRecords.push(record);
    this.cumulativeInput += BigInt(record.inputTokens);
    this.cumulativeOutput += BigInt(record.outputTokens);
    this.cumulativeCount += 1;
    if (this.allRecords.length > this.maxRecords) {
      this.allRecords.splice(0, this.allRecords.length - this.maxRecords);
    }
    if (this.buffer.length >= this.maxBufferSize) {
      // Fire and forget — errors are logged inside flushNow's catch path.
      this.flushNow().catch(() => {});
    }
  }

  getSummary(): UsageSummary {
    const providerMap = new Map<string, { input: bigint; output: bigint; count: number }>();
    for (const r of this.allRecords) {
      const e = providerMap.get(r.provider);
      if (e) {
        e.input += BigInt(r.inputTokens);
        e.output += BigInt(r.outputTokens);
        e.count += 1;
      } else {
        providerMap.set(r.provider, { input: BigInt(r.inputTokens), output: BigInt(r.outputTokens), count: 1 });
      }
    }
    const byProvider: ProviderSummary[] = [];
    for (const [provider, d] of providerMap) {
      byProvider.push({ provider, inputTokens: d.input, outputTokens: d.output, requestCount: d.count });
    }
    return { totalInputTokens: this.cumulativeInput, totalOutputTokens: this.cumulativeOutput, totalCost: 0, byProvider, recordCount: this.cumulativeCount };
  }

  /**
   * Drain the current buffer and hand it to the configured transport.
   *
   * Behaviour:
   *  - empty buffer: no-op (no transport call).
   *  - transport resolves: batch is considered delivered and discarded.
   *  - transport rejects: batch is re-queued at the front of the buffer
   *    (preserving at-least-once semantics) provided the total queue size
   *    would not exceed 2 * maxBufferSize. Otherwise the batch is dropped
   *    to prevent unbounded memory growth on a persistently-failing transport.
   */
  async flushNow(): Promise<void> {
    if (this.buffer.length === 0) return;
    const batch = this.buffer.splice(0);
    try {
      await this.transport(batch);
    } catch (err) {
      // Re-enqueue if there is headroom, otherwise drop to bound memory.
      if (this.buffer.length + batch.length <= 2 * this.maxBufferSize) {
        this.buffer.unshift(...batch);
      }
      console.error(
        JSON.stringify({
          event: "token_flush_failed",
          error: err instanceof Error ? err.message : String(err),
          batchSize: batch.length,
          bufferSize: this.buffer.length,
        }),
      );
      throw err;
    }
  }

  start(): void {
    setInterval(() => {
      this.flushNow().catch(() => {});
    }, this.flushIntervalMs);
  }
}
