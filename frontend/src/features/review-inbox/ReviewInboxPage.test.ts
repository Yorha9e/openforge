import { describe, expect, it } from 'vitest';
import { reviewLinkForItem } from './ReviewInboxPage';

describe('ReviewInboxPage', () => {
  it('links review items by project id and pipeline id', () => {
    expect(reviewLinkForItem({ project_id: 'proj-1', pipeline_id: 'pipe-1' })).toBe(
      '/project/proj-1/pipeline/pipe-1',
    );
  });
});
