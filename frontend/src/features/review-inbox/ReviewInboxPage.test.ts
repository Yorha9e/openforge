import { describe, expect, it } from 'vitest';
import { reviewLinkForItem, reviewTitleForItem, reviewWaitingSinceForItem } from './ReviewInboxPage';

describe('ReviewInboxPage', () => {
  it('links review items by project id and pipeline id', () => {
    expect(reviewLinkForItem({ project_id: 'proj-1', pipeline_id: 'pipe-1' })).toBe(
      '/project/proj-1/pipeline/pipe-1',
    );
  });

  it('formats review item title from project and pipeline context', () => {
    expect(reviewTitleForItem({
      project_name: 'Conduit',
      pipeline_title: 'Add tag filters',
      pipeline_id: 'pipe-1',
    })).toBe('Conduit / Add tag filters');
  });

  it('prefers awaiting_since for waiting time source', () => {
    expect(reviewWaitingSinceForItem({
      awaiting_since: '2026-06-04T00:00:00Z',
      created_at: '2026-06-03T00:00:00Z',
    })).toBe('2026-06-04T00:00:00Z');
  });
});
