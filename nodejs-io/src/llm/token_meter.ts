// llm/token_meter.ts — Ring buffer token counter

interface TokenRecord {
  pipelineId: string;
  projectId: string;
  provider: string;
  model: string;
  inputTokens: number;
  outputTokens: number;
  timestamp: Date;
}

export class TokenMeter {
  private buffer: TokenRecord[] = [];
  private readonly maxBufferSize = 500;
  private flushIntervalMs = 5000;

  record(record: TokenRecord): void {
    this.buffer.push(record);
    if (this.buffer.length >= this.maxBufferSize) {
      this.flush();
    }
  }

  private flush(): void {
    if (this.buffer.length === 0) return;
    const batch = this.buffer.splice(0);
    console.log(
      JSON.stringify({ event: "token_batch", count: batch.length, records: batch })
    );
  }

  start(): void {
    setInterval(() => this.flush(), this.flushIntervalMs);
  }
}
