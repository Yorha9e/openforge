import { describe, it, expect } from 'vitest';
import { simpleModeGridClass, viewModeFromSettings } from './SimpleModePage';

describe('SimpleModePage helpers', () => {
  it('uses a 60/40 two-column grid class', () => {
    // Tailwind arbitrary value: 3fr / 2fr is the 60/40 split (3:2 == 60:40)
    expect(simpleModeGridClass()).toMatch(/grid-cols-\[3fr_2fr\]|grid-cols-\[60fr_40fr\]/);
  });

  it('defaults to simple mode when settings missing or undefined', () => {
    expect(viewModeFromSettings(undefined)).toBe('simple');
    expect(viewModeFromSettings(null)).toBe('simple');
    expect(viewModeFromSettings({})).toBe('simple');
  });

  it('defaults to simple mode when defaultViewMode is not simple/pro', () => {
    expect(viewModeFromSettings({ layout: { defaultViewMode: 'auto' as any } })).toBe('simple');
  });

  it('honors an explicit simple defaultViewMode', () => {
    expect(viewModeFromSettings({ layout: { defaultViewMode: 'simple' } })).toBe('simple');
  });

  it('honors an explicit pro defaultViewMode', () => {
    expect(viewModeFromSettings({ layout: { defaultViewMode: 'pro' } })).toBe('pro');
  });
});
