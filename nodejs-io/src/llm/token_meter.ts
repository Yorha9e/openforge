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

/**
 * Path C T7: optional HTTP flush configuration.  When set, the meter POSTs
 * token batches to the given URL with an optional W3C traceparent header so
 * the downstream receiver can continue the current Go↔Node trace.  When
 * unset, the meter keeps its original console.log behaviour.
 */
export interface HTTPFlushConfig {
  url: string;
  /** Optional W3C traceparent string; when present it is sent verbatim. */
  traceparent?: string;
}

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

  /** Optional HTTP transport for forwarding batches.  Null = console.log only. */
  private httpFlushConfig: HTTPFlushConfig | null = null;

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

  /**
   * Configure (or clear) the HTTP flush transport.
   *
   * Pass `null` to revert to console.log-only flushing.  When the
   * transport's `traceparent` is set, that W3C header is included on
   * every outbound request so the receiver can continue the trace.
   */
  setFlushTransport(transport: HTTPFlushConfig | null): void {
    this.httpFlushConfig = transport;
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

  /**
   * Path C T7: optional HTTP flush path.  When `httpFlushConfig` is set,
   * the meter POSTs batches to its URL with an optional W3C traceparent
   * header so the downstream receiver can continue the current trace.
   * On transport failure the batch is logged locally so data is not lost.
   */
  private async flushHttp(): Promise<void> {
    if (!this.httpFlushConfig) return;
    if (this.buffer.length === 0) return;
    const batch = this.buffer.splice(0);
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };
    if (this.httpFlushConfig.traceparent) {
      headers["traceparent"] = this.httpFlushConfig.traceparent;
    }
    try {
      await fetch(this.httpFlushConfig.url, {
        method: "POST",
        headers,
        body: JSON.stringify({ event: "token_batch", count: batch.length, records: batch }),
      });
    } catch (err) {
      // On transport failure, log the batch locally so data is not lost.
      console.error("token_meter HTTP flush failed; logging locally", err);
      console.log(JSON.stringify({ event: "token_batch", count: batch.length, records: batch }));
    }
  }

  start(): void {
    setInterval(() => {
      this.flushNow().catch(() => {});
      // fire-and-forget; flushHttp() handles its own errors
      void this.flushHttp();
    }, this.flushIntervalMs);
  }
}
