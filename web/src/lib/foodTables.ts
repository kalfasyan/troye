import type { RecordModel } from 'pocketbase';

// The food tables Troye searches, and what each one's licence asks us to show.

export type TableName = 'usda' | 'nevo' | 'off';

/** An imported table, as the server records it in food_tables. */
export interface FoodTable extends RecordModel {
  name: TableName;
  version: string;
  foods: number;
  imported: string;
  running: boolean;
  error: string;
}

export const tableInfo: Record<
  TableName,
  { title: string; about: string; credit: string; link: string; download?: string }
> = {
  usda: {
    title: 'USDA FoodData Central',
    about: 'About 8,000 everyday foods, from fruit and vegetables to cooked meals, in English.',
    credit:
      'U.S. Department of Agriculture, Agricultural Research Service. FoodData Central. Public domain.',
    link: 'https://fdc.nal.usda.gov/',
  },
  nevo: {
    title: 'NEVO',
    about: 'About 2,300 Dutch and Flemish foods from RIVM, in Dutch and English.',
    credit: 'NEVO-online version 2025/9.0, RIVM, Bilthoven.',
    link: 'https://www.rivm.nl/en/dutch-food-composition-database',
    download:
      'https://www.rivm.nl/en/dutch-food-composition-database/use-of-nevo-online/request-dataset',
  },
  off: {
    title: 'Open Food Facts',
    about:
      'Packaged foods sold in Belgium and the Netherlands, kept on your server so barcodes work offline. Others are looked up online.',
    credit: 'Open Food Facts contributors. Open Database Licence.',
    link: 'https://world.openfoodfacts.org/',
  },
};

/** One line on how a table stands: its size and edition, or that it isn't in yet. */
export function tableStatus(t: FoodTable | undefined, now = new Date()): string {
  if (t?.running) return t.name === 'off' ? 'Downloading, this takes a while…' : 'Importing…';
  if (!t || !t.foods) return t?.error ? `Failed: ${t.error}` : 'Not imported yet';
  const count = `${t.foods.toLocaleString('en')} ${t.name === 'off' ? 'products' : 'foods'}`;
  const parts = [count];
  if (t.name === 'off') {
    if (t.imported) parts.push(`updated ${ago(new Date(t.imported.replace(' ', 'T')), now)}`);
  } else if (t.version) {
    parts.push(`edition ${t.version}`);
  }
  if (t.error) parts.push(`last update failed: ${t.error}`);
  return parts.join(' · ');
}

function ago(d: Date, now: Date): string {
  const days = Math.floor((now.getTime() - d.getTime()) / 86_400_000);
  if (days <= 0) return 'today';
  if (days === 1) return 'yesterday';
  if (days < 60) return `${days} days ago`;
  return `on ${d.toLocaleDateString('en', { day: 'numeric', month: 'short', year: 'numeric' })}`;
}
