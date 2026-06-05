import { describe, it, expect, beforeEach } from "vitest";
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
});
