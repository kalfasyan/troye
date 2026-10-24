import { describe, expect, it } from 'vitest';
import { errorMessage } from './pb';

describe('errorMessage', () => {
  it("uses the server's message", () => {
    expect(errorMessage({ status: 400, response: { message: 'Too short.' } })).toBe('Too short.');
  });

  it('explains an unreachable server', () => {
    expect(errorMessage({ status: 0, response: {} })).toBe(
      "Can't reach the server. Check your connection.",
    );
  });

  it('falls back to the given text', () => {
    expect(errorMessage(new Error('boom'), 'Wrong email or password.')).toBe(
      'Wrong email or password.',
    );
  });
});
