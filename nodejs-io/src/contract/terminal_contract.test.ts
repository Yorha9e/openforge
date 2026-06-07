import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  OpenTerminalRequestSchema,
  TerminalOutputSchema,
  type OpenTerminalRequest,
  type TerminalOutput,
} from "../gen/agent/v1/terminal_pb";
import { loadGolden } from "./golden_loader.js";

describe("Terminal Contract (Open)", () => {
  it("OpenTerminalRequest maps golden fields onto camelCase logical fields", () => {
    const golden = loadGolden("terminal_open.req.json") as {
      pipeline_id: string;
      container_id: string;
      mode: string;
      actor: string;
    };

    const req: OpenTerminalRequest = create(OpenTerminalRequestSchema, {
      pipelineId: golden.pipeline_id,
      containerId: golden.container_id,
      mode: golden.mode as OpenTerminalRequest["mode"],
      actor: golden.actor,
    });

    expect(req.pipelineId).toBe("p-001");
    expect(req.containerId).toBe("ctr-abc");
    expect(req.mode).toBe("TERMINAL_MODE_READ_ONLY");
    expect(req.actor).toBe("user@example.com");
  });

  it("TerminalOutput maps golden fields onto camelCase logical fields", () => {
    const golden = loadGolden("terminal_open.resp.json") as {
      stream: string;
      data: string; // base64-encoded bytes
      is_done: boolean;
    };

    const data = new Uint8Array(Buffer.from(golden.data, "base64"));

    const resp: TerminalOutput = create(TerminalOutputSchema, {
      stream: golden.stream,
      data,
      isDone: golden.is_done,
    });

    expect(resp.stream).toBe("stdout");
    expect(Buffer.from(resp.data).toString()).toBe("hello\n");
    expect(resp.isDone).toBe(false);
  });
});
