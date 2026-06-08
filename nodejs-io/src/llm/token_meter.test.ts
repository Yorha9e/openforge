import { describe, it, expect, vi, beforeEach } from "vitest";
import { TokenMeter } from "./token_meter.js";

describe("TokenMeter", () => {
  let meter: TokenMeter;
  beforeEach(() => { meter = new TokenMeter(); });

  it("should return zero summary when empty", () => {
    const s = meter.getSummary();
    expect(s.totalInputTokens).toBe(0n);
    expect(s.totalOutputTokens).toBe(0n);
    expect(s.byProvider).toEqual([]);
  });

  it("should aggregate tokens across providers", () => {
    meter.record({ pipelineId: "p1", projectId: "", provider: "anthropic", model: "claude", inputTokens: 100, outputTokens: 50, timestamp: new Date() });
    meter.record({ pipelineId: "p1", projectId: "", provider: "openai", model: "gpt4", inputTokens: 200, outputTokens: 80, timestamp: new Date() });
    meter.record({ pipelineId: "p2", projectId: "", provider: "anthropic", model: "haiku", inputTokens: 300, outputTokens: 120, timestamp: new Date() });
    const s = meter.getSummary();
    expect(s.totalInputTokens).toBe(600n);
    expect(s.totalOutputTokens).toBe(250n);
    expect(s.byProvider.length).toBe(2);
    const a = s.byProvider.find((p) => p.provider === "anthropic");
    expect(a?.inputTokens).toBe(400n);
  });

  it("should cap allRecords at maxRecords and maintain cumulative totals", () => {
    const totalRecords = 15_000;
    const inputPerRecord = 10;
    const outputPerRecord = 5;
    for (let i = 0; i < totalRecords; i++) {
      meter.record({
        pipelineId: "p1",
        projectId: "",
        provider: i % 2 === 0 ? "anthropic" : "openai",
        model: "test",
        inputTokens: inputPerRecord,
        outputTokens: outputPerRecord,
        timestamp: new Date(),
      });
    }
    const s = meter.getSummary();
    // Cumulative counters must reflect ALL 15000 records
    expect(s.recordCount).toBe(totalRecords);
    expect(s.totalInputTokens).toBe(BigInt(totalRecords * inputPerRecord));
    expect(s.totalOutputTokens).toBe(BigInt(totalRecords * outputPerRecord));
    // allRecords is capped at 10000 — access via cast for testing
    const internal = meter as unknown as { allRecords: unknown[] };
    expect(internal.allRecords.length).toBeLessThanOrEqual(10_000);
    // byProvider should still work on the retained window
    expect(s.byProvider.length).toBe(2);
  });
});

describe("TokenMeter.flushNow()", () => {
  it("calls injected transport with buffered records", async () => {
    const sentPayloads: unknown[][] = [];
    const transport = vi.fn(async (batch) => {
      sentPayloads.push([...batch]);
    });
    const m = new TokenMeter(transport);
    m.record({ pipelineId: "p1", projectId: "pr1", provider: "anthropic", model: "claude-sonnet-4-6", inputTokens: 10, outputTokens: 20, timestamp: new Date() });
    m.record({ pipelineId: "p1", projectId: "pr1", provider: "anthropic", model: "claude-sonnet-4-6", inputTokens: 30, outputTokens: 40, timestamp: new Date() });

    await m.flushNow();

    expect(transport).toHaveBeenCalledTimes(1);
    expect(sentPayloads).toHaveLength(1);
    expect(sentPayloads[0]).toHaveLength(2);
  });

  it("is no-op when buffer empty", async () => {
    const transport = vi.fn(async () => {});
    const m = new TokenMeter(transport);
    await m.flushNow();
    expect(transport).not.toHaveBeenCalled();
  });
});

