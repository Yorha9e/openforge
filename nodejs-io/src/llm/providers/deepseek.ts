// llm/providers/deepseek.ts — DeepSeek provider (Anthropic Messages API compatible)

import { AnthropicProvider } from "./anthropic.js";

// DeepSeek API is Anthropic Messages API compatible.
// Reuse AnthropicProvider with a different base URL.
export class DeepSeekProvider extends AnthropicProvider {
  constructor(baseURL: string, apiKey: string) {
    super(apiKey, baseURL);
  }
}
