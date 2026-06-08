import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  ChatRequestSchema,
  ChatEventSchema,
  type ChatRequest,
  type ChatEvent,
} from "../gen/agent/v1/coordinator_pb";
import { loadGolden } from "./golden_loader.js";

describe("LLM Contract (ChatRequest / ChatEvent)", () => {
  it("ChatRequest maps golden snake_case fields onto logical camelCase fields", () => {
    const golden = loadGolden("llm_chat.req.json") as {
      pipeline_id: string;
      branch_id: string;
      messages: { role: string; content: string; msg_seq: number }[];
    };

    const req: ChatRequest = create(ChatRequestSchema, {
      pipelineId: golden.pipeline_id,
      branchId: golden.branch_id,
      messages: golden.messages.map((m) => ({
        role: m.role as ChatRequest["messages"][number]["role"],
        content: m.content,
        msgSeq: m.msg_seq,
      })),
    });

    expect(req.pipelineId).toBe("p-001");
    expect(req.branchId).toBe("b-main");
    expect(req.messages).toHaveLength(1);
    expect(req.messages[0].role).toBe("CHAT_ROLE_USER");
    expect(req.messages[0].content).toBe("Hello");
    expect(req.messages[0].msgSeq).toBe(1);
  });

  it("ChatEvent maps golden snake_case fields onto logical camelCase fields", () => {
    const golden = loadGolden("llm_chat.resp.json") as {
      msg_seq: number;
      event_type: string;
      delta_text: string;
      is_done: boolean;
    };

    const ev: ChatEvent = create(ChatEventSchema, {
      msgSeq: golden.msg_seq,
      eventType: golden.event_type as ChatEvent["eventType"],
      deltaText: golden.delta_text,
      isDone: golden.is_done,
    });

    expect(ev.msgSeq).toBe(1);
    expect(ev.eventType).toBe("CHAT_EVENT_TYPE_DELTA");
    expect(ev.deltaText).toBe("Hi");
    expect(ev.isDone).toBe(false);
  });
});
