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

    const resp = await this.client.messages.create({
      model: req.config.model,
      max_tokens: req.config.maxTokens ?? 4096,
      temperature: req.config.temperature,
      system: systemMessages.length > 0 ? systemMessages : undefined,
      messages: userMessages,
    });

    const textBlock = resp.content.find((b) => b.type === "text");
    const content = textBlock && textBlock.type === "text" ? textBlock.text : "";

    return {
      id: resp.id,
      content,
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
