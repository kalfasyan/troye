import { describe, expect, it } from 'vitest';
import { isIos } from './platform';

describe('isIos', () => {
  it('detects iPhones', () => {
    expect(isIos('Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)')).toBe(true);
  });

  it('detects iPads that report themselves as Macs', () => {
    expect(isIos('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)', 5)).toBe(true);
  });

  it('leaves desktop Macs and Android alone', () => {
    expect(isIos('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)', 0)).toBe(false);
    expect(isIos('Mozilla/5.0 (Linux; Android 15; Pixel 9)', 5)).toBe(false);
  });
});
