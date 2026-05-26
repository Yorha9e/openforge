import { describe, it, expect } from 'vitest';
import { api, setToken } from './api';

describe('API Client', () => {
  it('should export api object with expected methods', () => {
    expect(api).toBeDefined();
    expect(typeof api.login).toBe('function');
    expect(typeof api.listProjects).toBe('function');
    expect(typeof api.getPipeline).toBe('function');
    expect(typeof api.getHealth).toBe('function');
  });

  it('should export setToken function', () => {
    expect(typeof setToken).toBe('function');
  });
});
