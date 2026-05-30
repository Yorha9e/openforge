// llm/providers/anthropic.ts — Anthropic Messages API adapter

import Anthropic from "@anthropic-ai/sdk";
import type { LLMProvider, ChatRequest, ChatResponse } from "../../kernel/interfaces.js";

export class AnthropicProvider implements LLMProvider {
  private client: Anthropic;

  constructor(apiKey: string, baseURL?: string) {
    this.client = new Anthropic({ apiKey, baseURL });
  }

  async chat(req: ChatRequest): Promise<ChatResponse> {
    const systemMessages = req.messages
      .filter((m) => m.role === "system")
      .map((m) => ({ type: "text" as const, text: m.content }));

    const userMessages = req.messages
      .filter((m) => m.role !== "system")
      .map((m) => ({
        role: m.role === "agent" ? ("assistant" as const) : ("user" as const),
        content: m.content,
      }));

    // Build tools array for Anthropic API
    const tools = req.tools?.map((t) => ({
      name: t.name,
      description: t.description,
      input_schema: {
        type: "object" as const,
        ...t.inputSchema
      }
    }));

    const resp = await this.client.messages.create({
      model: req.config.model,
      max_tokens: req.config.maxTokens ?? 4096,
      temperature: req.config.temperature,
      system: systemMessages.length > 0 ? systemMessages : undefined,
      messages: userMessages,
      tools: tools && tools.length > 0 ? tools : undefined,
    });

    // Serialize ALL content blocks as JSON so Go side can parse tool_use blocks
    const contentJson = JSON.stringify(resp.content);

    return {
      id: resp.id,
      content: contentJson,
      stopReason: resp.stop_reason ?? "end_turn",
      usage: {
        inputTokens: resp.usage.input_tokens,
        outputTokens: resp.usage.output_tokens,
      },
    };
  }

  async *chatStream(req: ChatRequest): AsyncIterable<string> {
    const systemMessages = req.messages
      .filter((m) => m.role === "system")
      .map((m) => ({ type: "text" as const, text: m.content }));

    const userMessages = req.messages
      .filter((m) => m.role !== "system")
      .map((m) => ({
        role: m.role === "agent" ? ("assistant" as const) : ("user" as const),
        content: m.content,
      }));

    const stream = await this.client.messages.create({
      model: req.config.model,
      max_tokens: req.config.maxTokens ?? 4096,
      temperature: req.config.temperature,
      system: systemMessages.length > 0 ? systemMessages : undefined,
      messages: userMessages,
      stream: true,
    });

    for await (const event of stream) {
      if (
        event.type === "content_block_delta" &&
        event.delta.type === "text_delta"
      ) {
        yield event.delta.text;
      }
    }
  }
}
