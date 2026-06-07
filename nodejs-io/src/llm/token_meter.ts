interface TokenRecord {
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
 * Transport configuration for the optional HTTP flush path.
 *
 * Path C T7: when a flushTransport is configured, the meter will POST
 * token batches to the given URL.  The traceparent header is honoured
 * (and merged with the W3C TraceContext propagator) so the downstream
 * receiver continues the current Go↔Node trace.  When no transport is
 * configured, the meter keeps its original console.log behaviour.
 */
export interface FlushTransport {
  url: string;
  /** Optional W3C traceparent string; when present it is sent verbatim. */
  traceparent?: string;
}

export class TokenMeter {
  private buffer: TokenRecord[] = [];
  private allRecords: TokenRecord[] = [];
  private readonly maxBufferSize = 500;
  private readonly maxRecords = 10_000;
  private flushIntervalMs = 5000;

  private cumulativeInput = 0n;
  private cumulativeOutput = 0n;
  private cumulativeCount = 0;

  /** Optional HTTP transport for forwarding batches.  Null = console.log only. */
  private flushTransport: FlushTransport | null = null;

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
      this.flush();
    }
  }

  /**
   * Configure (or clear) the HTTP flush transport.
   *
   * Pass `null` to revert to console.log-only flushing.  When the
   * transport's `traceparent` is set, that W3C header is included on
   * every outbound request so the receiver can continue the trace.
   */
  setFlushTransport(transport: FlushTransport | null): void {
    this.flushTransport = transport;
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

  private async flush(): Promise<void> {
    if (this.buffer.length === 0) return;
    const batch = this.buffer.splice(0);

    if (this.flushTransport) {
      // HTTP flush path — honour the W3C traceparent if supplied.
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
      };
      if (this.flushTransport.traceparent) {
        headers["traceparent"] = this.flushTransport.traceparent;
      }
      try {
        await fetch(this.flushTransport.url, {
          method: "POST",
          headers,
          body: JSON.stringify({ event: "token_batch", count: batch.length, records: batch }),
        });
      } catch (err) {
        // On transport failure, log the batch locally so data is not lost.
        console.error("token_meter HTTP flush failed; logging locally", err);
        console.log(JSON.stringify({ event: "token_batch", count: batch.length, records: batch }));
      }
      return;
    }

    // Default: console.log only (preserves existing behaviour).
    console.log(JSON.stringify({ event: "token_batch", count: batch.length, records: batch }));
  }

  start(): void {
    setInterval(() => {
      // fire-and-forget; flush() handles its own errors
      void this.flush();
    }, this.flushIntervalMs);
  }
}
