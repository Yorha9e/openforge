// llm/providers/openai.ts — OpenAI Chat Completions API adapter

import type { LLMProvider, ChatRequest, ChatResponse } from "../../kernel/interfaces.js";

export class OpenAIProvider implements LLMProvider {
  private baseURL: string;
  private apiKey: string;

  constructor(baseURL: string, apiKey: string) {
    this.baseURL = baseURL;
    this.apiKey = apiKey;
  }

  async chat(req: ChatRequest): Promise<ChatResponse> {
    const body = this.buildBody(req, false);
    const resp = await fetch(`${this.baseURL}/v1/chat/completions`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${this.apiKey}`,
      },
      body: JSON.stringify(body),
    });

    if (!resp.ok) {
      const text = await resp.text();
      throw new Error(`openai: ${resp.status} ${text}`);
    }

    const json = await resp.json() as any;
    const choice = json.choices?.[0];
    return {
      id: json.id ?? "",
      content: choice?.message?.content ?? "",
      usage: {
        inputTokens: json.usage?.prompt_tokens ?? 0,
        outputTokens: json.usage?.completion_tokens ?? 0,
      },
    };
  }

  async *chatStream(req: ChatRequest): AsyncIterable<string> {
    const body = this.buildBody(req, true);
    const resp = await fetch(`${this.baseURL}/v1/chat/completions`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${this.apiKey}`,
      },
      body: JSON.stringify(body),
    });

    if (!resp.ok) {
      const text = await resp.text();
      throw new Error(`openai stream: ${resp.status} ${text}`);
    }

    const reader = resp.body?.getReader();
    if (!reader) return;

    const decoder = new TextDecoder();
    let buffer = "";
    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() ?? "";
        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed.startsWith("data: ")) continue;
          const data = trimmed.slice(6);
          if (data === "[DONE]") return;
          try {
            const ev = JSON.parse(data);
            const delta = ev.choices?.[0]?.delta?.content;
            if (delta) yield delta;
          } catch {
            // skip malformed lines
          }
        }
      }
    } finally {
      reader.releaseLock();
    }
  }

  private buildBody(req: ChatRequest, stream: boolean): Record<string, unknown> {
    const messages: Array<{ role: string; content: string }> = [];
    for (const m of req.messages) {
      messages.push({ role: m.role === "agent" ? "assistant" : m.role, content: m.content });
    }
    return {
      model: req.config.model,
      messages,
      max_tokens: req.config.maxTokens ?? 4096,
      temperature: req.config.temperature,
      stream,
    };
  }
}
