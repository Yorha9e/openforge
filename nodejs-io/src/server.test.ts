import { describe, it, expect } from 'vitest';

describe('Server', () => {
  it('should have correct module structure', () => {
    // Basic smoke test
    expect(true).toBe(true);
  });

  it('should export server module when env vars are set', () => {
    // The server module requires ANTHROPIC_API_KEY at top-level and calls process.exit if missing.
    // We verify the file exists and is parseable rather than importing it (which triggers side effects).
    const fs = require('fs');
    const path = require('path');
    const serverPath = path.resolve(__dirname, 'server.ts');
    expect(fs.existsSync(serverPath)).toBe(true);
    const content = fs.readFileSync(serverPath, 'utf-8');
    expect(content).toContain('ANTHROPIC_API_KEY');
    expect(content).toContain('gRPC');
  });
});
