// Node-side helpers for gRPC contract tests.
// These mirror the Go-side test/contract/golden_loader.go.
//
// The golden JSON files in this directory are the canonical contract:
// they are identical to test/contract/golden/ on the Go side and must
// round-trip through each runtime's own (un)marshal logic.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = __filename.replace(/[/\\][^/\\]+$/, "");

export function goldenDir(): string {
  return join(__dirname, "golden");
}

export function loadGolden(name: string): Record<string, unknown> {
  const raw = readFileSync(join(goldenDir(), name), "utf-8");
  return JSON.parse(raw) as Record<string, unknown>;
}
