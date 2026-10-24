import { describe, expect, it } from 'vitest';
import { tableStatus, type FoodTable } from './foodTables';

const table = (extra: Partial<FoodTable>) =>
  ({
    name: 'ciqual',
    version: '',
    foods: 0,
    imported: '',
    running: false,
    error: '',
    ...extra,
  }) as FoodTable;

describe('tableStatus', () => {
  const now = new Date('2026-10-24T12:00:00Z');

  it('says when a table is missing', () => {
    expect(tableStatus(undefined)).toBe('Not imported yet');
    expect(tableStatus(table({ name: 'nevo', error: 'no ENERCC column' }))).toBe(
      'Failed: no ENERCC column',
    );
  });

  it('gives the size and edition', () => {
    expect(tableStatus(table({ foods: 3185, version: '2025-11-03' }))).toBe(
      '3,185 foods · edition 2025',
    );
    expect(tableStatus(table({ name: 'nevo', foods: 2328, version: '2025/9.0' }))).toBe(
      '2,328 foods · edition 2025/9.0',
    );
  });

  it('says how fresh the products are', () => {
    const off = table({ name: 'off', foods: 120500, imported: '2026-10-21 04:40:00.000Z' });
    expect(tableStatus(off, now)).toBe('120,500 products · updated 3 days ago');
    expect(tableStatus({ ...off, running: true }, now)).toBe('Downloading, this takes a while…');
    expect(tableStatus({ ...off, error: 'status 503' }, now)).toBe(
      '120,500 products · updated 3 days ago · last update failed: status 503',
    );
  });
});
