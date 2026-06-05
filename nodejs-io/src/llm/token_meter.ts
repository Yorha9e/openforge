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

export class TokenMeter {
  private buffer: TokenRecord[] = [];
  private allRecords: TokenRecord[] = [];
  private readonly maxBufferSize = 500;
  private flushIntervalMs = 5000;

  record(record: TokenRecord): void {
    this.buffer.push(record);
    this.allRecords.push(record);
    if (this.buffer.length >= this.maxBufferSize) {
      this.flush();
    }
  }

  getSummary(): UsageSummary {
    const providerMap = new Map<string, { input: bigint; output: bigint; count: number }>();
    let totalInput = 0n;
    let totalOutput = 0n;
    for (const r of this.allRecords) {
      totalInput += BigInt(r.inputTokens);
      totalOutput += BigInt(r.outputTokens);
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
    return { totalInputTokens: totalInput, totalOutputTokens: totalOutput, totalCost: 0, byProvider, recordCount: this.allRecords.length };
  }

  private flush(): void {
    if (this.buffer.length === 0) return;
    const batch = this.buffer.splice(0);
    console.log(JSON.stringify({ event: "token_batch", count: batch.length, records: batch }));
  }

  start(): void {
    setInterval(() => this.flush(), this.flushIntervalMs);
  }
}
