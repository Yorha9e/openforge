import { describe, it, expect } from 'vitest';
import { AnthropicProvider } from './anthropic.js';

describe('AnthropicProvider', () => {
  it('should create provider with API key', () => {
    const provider = new AnthropicProvider('test-api-key');
    expect(provider).toBeDefined();
  });

  it('should create provider with custom base URL', () => {
    const provider = new AnthropicProvider('test-api-key', 'https://custom.api.com');
    expect(provider).toBeDefined();
  });
});
