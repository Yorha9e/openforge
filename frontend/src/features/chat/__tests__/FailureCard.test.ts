import { describe, it, expect } from 'vitest';
import { failureCardTitle, failureCardAdvice, failureCardSeverity, FAILURE_CODES } from '../FailureCard';

describe('FailureCard helpers', () => {
  it.each([
    'MODEL_HALLUCINATION',
    'PROMPT_WEAKNESS',
    'DEPENDENCY_CONFLICT',
    'SANDBOX_TIMEOUT',
    'REPO_BUG',
    'CONTEXT_OVERFLOW',
    'TOKEN_QUOTA_EXCEEDED',
    'UNKNOWN',
  ] as const)('exposes a friendly title for %s', (code) => {
    const title = failureCardTitle(code);
    expect(title).toBeTruthy();
    expect(typeof title).toBe('string');
    expect(title.length).toBeGreaterThan(0);
  });

  it.each([
    ['MODEL_HALLUCINATION', 'rephrasing'],
    ['PROMPT_WEAKNESS', 'context or constraints'],
    ['DEPENDENCY_CONFLICT', 'package.json'],
    ['SANDBOX_TIMEOUT', 'smaller steps'],
    ['REPO_BUG', 'git status'],
    ['CONTEXT_OVERFLOW', 'new branch'],
    ['TOKEN_QUOTA_EXCEEDED', 'next month'],
    ['UNKNOWN', 'contact support'],
  ] as const)('returns advice text for %s', (code, snippet) => {
    const advice = failureCardAdvice(code);
    expect(advice).toBeTruthy();
    expect(advice.toLowerCase()).toContain(snippet.toLowerCase());
  });

  it('falls back to UNKNOWN for unrecognized failure codes', () => {
    expect(failureCardTitle('NOT_A_REAL_CODE')).toBe(failureCardTitle('UNKNOWN'));
    expect(failureCardAdvice('NOT_A_REAL_CODE')).toBe(failureCardAdvice('UNKNOWN'));
    expect(failureCardSeverity('NOT_A_REAL_CODE')).toBe(failureCardSeverity('UNKNOWN'));
  });

  it('returns a severity of warn or error for every known code', () => {
    for (const code of FAILURE_CODES) {
      const severity = failureCardSeverity(code);
      expect(['warn', 'error']).toContain(severity);
    }
  });

  it('marks DEPENDENCY_CONFLICT, SANDBOX_TIMEOUT, REPO_BUG, TOKEN_QUOTA_EXCEEDED as error severity', () => {
    expect(failureCardSeverity('DEPENDENCY_CONFLICT')).toBe('error');
    expect(failureCardSeverity('SANDBOX_TIMEOUT')).toBe('error');
    expect(failureCardSeverity('REPO_BUG')).toBe('error');
    expect(failureCardSeverity('TOKEN_QUOTA_EXCEEDED')).toBe('error');
  });

  it('marks the rest of the known codes as warn severity', () => {
    expect(failureCardSeverity('MODEL_HALLUCINATION')).toBe('warn');
    expect(failureCardSeverity('PROMPT_WEAKNESS')).toBe('warn');
    expect(failureCardSeverity('CONTEXT_OVERFLOW')).toBe('warn');
    expect(failureCardSeverity('UNKNOWN')).toBe('warn');
  });
});
