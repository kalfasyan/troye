import { describe, expect, it } from 'vitest';
import { pushProblem } from './pushProblem';

describe('pushProblem', () => {
  it('tells Brave users where its push switch is', () => {
    expect(pushProblem('subscribe', true)).toContain('brave://settings/privacy');
  });

  it('includes the browser error when there is one', () => {
    expect(pushProblem('subscribe', false, 'AbortError')).toContain('(AbortError)');
  });

  it('explains blocked permission per browser', () => {
    expect(pushProblem('permission-denied', false)).toContain('allow Notifications');
    expect(pushProblem('permission-denied', true)).toContain('Brave is blocking');
  });
});
