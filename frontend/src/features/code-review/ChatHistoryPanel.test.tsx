import { describe, it, expect } from 'vitest';
import { canActivateBranch, MAX_ACTIVE_BRANCHES, type BranchLike } from './ChatHistoryPanel';

const fourBranches: BranchLike[] = [
  { id: 'b1', status: 'active' },
  { id: 'b2', status: 'active' },
  { id: 'b3', status: 'active' },
  { id: 'b4', status: 'abandoned' },
];

const threeActiveTwoInactive: BranchLike[] = [
  { id: 'b1', status: 'active' },
  { id: 'b2', status: 'active' },
  { id: 'b3', status: 'abandoned' },
  { id: 'b4', status: 'abandoned' },
];

const twoActiveOneAbandoned: BranchLike[] = [
  { id: 'b1', status: 'active' },
  { id: 'b2', status: 'active' },
  { id: 'b3', status: 'abandoned' },
];

describe('canActivateBranch', () => {
  it('blocks activating a 4th branch when 3 are already active', () => {
    const result = canActivateBranch(fourBranches, 'b4');
    expect(result.allowed).toBe(false);
    expect(result.reason).toMatch(/3 active branches/);
    expect(result.reason).toMatch(/max 3/);
  });

  it('exposes the max active branch constant as 3', () => {
    expect(MAX_ACTIVE_BRANCHES).toBe(3);
  });

  it('allows switching to a currently active branch (no-op) when at the cap', () => {
    // b1/b2/b3 active, clicking b1 should still be allowed (just keeps it active)
    const result = canActivateBranch(fourBranches, 'b1');
    expect(result.allowed).toBe(true);
  });

  it('allows activating a dormant branch when active count drops below cap', () => {
    // b1/b2 active, b3 abandoned. Clicking b3 should be allowed.
    const result = canActivateBranch(twoActiveOneAbandoned, 'b3');
    expect(result.allowed).toBe(true);
  });

  it('allows activating a dormant 4th branch when only 2 are active', () => {
    const result = canActivateBranch(threeActiveTwoInactive, 'b4');
    expect(result.allowed).toBe(true);
  });

  it('counts merged and abandoned as not active toward the cap', () => {
    // 3 active, 2 abandoned: activating another abandoned is blocked
    const result = canActivateBranch(fourBranches, 'b4');
    expect(result.allowed).toBe(false);
  });

  it('returns allowed when target branch id does not exist (defensive)', () => {
    const result = canActivateBranch(fourBranches, 'does-not-exist');
    expect(result.allowed).toBe(true);
  });
});
