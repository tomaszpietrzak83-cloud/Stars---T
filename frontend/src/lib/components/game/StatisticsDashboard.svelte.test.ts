import { create } from '@bufbuild/protobuf';
import { expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { PlayerStatsSchema, PlayerAccountingSchema, StatisticsCountersSchema, StatisticsYearSchema } from '#lib/types/cs-proto.js';
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
