import { describe, expect, it } from 'vitest';
import { initialDemoViewFromSearch } from './demoView';

describe('7E2C bounded local initial-screen link', () => {
  it('opens the existing planning page from one exact query parameter', () => {
    expect(initialDemoViewFromSearch('?view=containment')).toBe('containment');
  });
  it('defaults to Overview instead of promoting arbitrary or malformed inputs', () => {
    for (const search of ['', '?utm_source=example', '?view=proof', '?view=environment',
      '?view=containment&view=containment', '?view=containment&approve=true',
      '?view=historical', '?view=containment%2Fhistorical', '?VIEW=containment']) {
      expect(initialDemoViewFromSearch(search)).toBe('overview');
    }
  });
  it('only selects a local UI page; approval is still entirely server-side', () => {
    expect(initialDemoViewFromSearch('?view=containment')).not.toBe('historical');
    expect(initialDemoViewFromSearch('?view=containment')).not.toBe('approved');
  });
});
