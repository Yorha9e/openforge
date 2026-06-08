import { describe, it, expect } from "vitest";
import { getError503Config, type Error503Reason } from "./ErrorPage";

describe("ErrorPage 503", () => {
  it("renders circuit breaker message when reason=circuit_open", () => {
    const config = getError503Config("circuit_open", 120);
    expect(config).toBeTruthy();
    expect(config!.title).toMatch(/Agent engine temporarily degraded/i);
    expect(config!.message).toContain("120");
  });

  it("renders quota message when reason=quota_exhausted", () => {
    const config = getError503Config("quota_exhausted");
    expect(config).toBeTruthy();
    expect(config!.title).toMatch(/Monthly quota exhausted/i);
  });

  it("returns null when no reason is supplied (caller falls back to legacy 503)", () => {
    const config = getError503Config(undefined);
    expect(config).toBeNull();
  });

  it("uses a 60s default retry window when retryAfter is omitted for circuit_open", () => {
    const config = getError503Config("circuit_open");
    expect(config).toBeTruthy();
    expect(config!.message).toContain("60");
  });

  it("returns null for an unknown 503 reason", () => {
    const config = getError503Config("not_a_real_reason" as Error503Reason);
    expect(config).toBeNull();
  });
});
