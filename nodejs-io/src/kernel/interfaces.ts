// kernel/interfaces.ts — Node-side port interfaces

export interface LLMProvider {
  chat(req: ChatRequest): Promise<ChatResponse>;
  chatStream(req: ChatRequest): AsyncIterable<string>;
}

export interface ChatRequest {
  messages: Message[];
  config: LLMConfig;
  tools?: Tool[];
}

export interface Message {
  role: string;
  content: string;
}

export interface LLMConfig {
  provider: string;
  model: string;
  apiEndpoint?: string;
  apiKey: string;
  maxTokens?: number;
  temperature?: number;
}

export interface ChatResponse {
  id: string;
  content: string;
  usage: { inputTokens: number; outputTokens: number };
}

export interface Tool {
  name: string;
  description: string;
  inputSchema: Record<string, unknown>;
}
