import { create } from '@bufbuild/protobuf';
import { expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { HullStatisticsSchema, PlayerStatsSchema, PlayerAccountingSchema, StatisticsCountersSchema, StatisticsYearSchema } from '#lib/types/cs-proto.js';
import StatisticsDashboard from './StatisticsDashboard.svelte';
it('explains tracking boundaries for older saves', async()=>{
 render(StatisticsDashboard,{stats:create(PlayerStatsSchema,{tokensBuilt:12,starbasesBuilt:2,planetsColonized:3})});
 await expect.element(page.getByText('Detailed tracking starts with the next generated turn.',{exact:false})).toBeVisible();
 await expect.element(page.getByText('No completed year recorded yet.')).toBeVisible();
});
it('renders exact large counters and newest yearly results first', async()=>{
 const counters=create(StatisticsCountersSchema,{shipsDestroyed:9n,shipsLost:2n,resourcesProduced:9007199254740993n,ironiumMined:7n});
 render(StatisticsDashboard,{stats:create(PlayerStatsSchema,{accounting:create(PlayerAccountingSchema,{sinceYear:2478,counters,history:[create(StatisticsYearSchema,{year:2478,counters}),create(StatisticsYearSchema,{year:2479,counters})]})})});
 await expect.element(page.getByText('9,007,199,254,740,993',{exact:true})).toBeVisible();
 await expect.element(page.getByText('2478',{exact:true}).first()).toBeVisible();
 const rows=page.getByRole('row');
 await expect.element(rows.nth(1)).toHaveTextContent('2479');
 await expect.element(rows.nth(2)).toHaveTextContent('2478');
});

it('expands ship counts and tonnage by hull and keeps older totals visible', async () => {
 const counters = create(StatisticsCountersSchema, {
  shipsDestroyed: 7n,
  destroyedTonnage: 300n,
  shipsLost: 1n,
  lostTonnage: 60n,
  shipsBuilt: 2n,
  builtTonnage: 40n,
  mineralsSalvaged: 45n,
  ironiumSalvaged: 30n,
  boraniumSalvaged: 10n,
  hulls: {
   Destroyer: create(HullStatisticsSchema, { shipsDestroyed: 4n, destroyedTonnage: 240n, shipsLost: 1n, lostTonnage: 60n }),
   Frigate: create(HullStatisticsSchema, { shipsBuilt: 2n, builtTonnage: 40n })
  }
 });
 render(StatisticsDashboard, { stats: create(PlayerStatsSchema, { accounting: create(PlayerAccountingSchema, { counters }) }) });
 await expect.element(page.getByText('Destroyer', { exact: true }).first()).not.toBeVisible();
 await page.getByText('Enemy ships destroyed', { exact: true }).click();
 await expect.element(page.getByText('Destroyer', { exact: true }).first()).toBeVisible();
 await expect.element(page.getByText('Hull not recorded', { exact: true }).first()).toBeVisible();
 await page.getByText('Built ship tonnage', { exact: true }).click();
 await expect.element(page.getByText('Frigate', { exact: true }).last()).toBeVisible();
 await expect.element(page.getByText('40 kT', { exact: true }).last()).toBeVisible();
 await expect.element(page.getByText('Earlier salvage, source unclassified')).toBeVisible();
});

it('shows the mineral spending example after a one-second hover', async () => {
 render(StatisticsDashboard, {});
 const button = page.getByRole('button', { name: 'Help: Minerals spent' });
 await button.hover();
 await expect.element(page.getByRole('tooltip')).not.toBeInTheDocument();
 const tip = page.getByRole('tooltip');
 await expect.element(tip).toBeVisible();
 await expect.element(tip).toHaveTextContent('Spending reaches 150 kT');
 await page.getByText('Mining and salvage', { exact: true }).hover();
 await expect.element(tip).not.toBeInTheDocument();
});
