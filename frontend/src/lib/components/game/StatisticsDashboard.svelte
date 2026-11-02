<script lang="ts">
 import type { PlayerStats, StatisticsCounters } from '#lib/types/cs-proto.js';
 let {stats}:{stats?:PlayerStats}=$props();
 type Row=[keyof StatisticsCounters,string,string?];
 const groups:{title:string;rows:Row[]}[]=[
  {title:'Combat',rows:[['shipsDestroyed','Enemy ships destroyed'],['destroyedTonnage','Enemy tonnage destroyed','kT'],['shipsLost','Own ships lost'],['lostTonnage','Own tonnage lost','kT'],['battlesFought','Battles fought'],['battlesWon','Battles won']]},
  {title:'Construction and recycling',rows:[['shipsBuilt','Ships built'],['builtTonnage','Built ship tonnage','kT'],['factoriesBuilt','Factories built'],['minesBuilt','Mines built'],['defensesBuilt','Defenses built'],['shipsScrapped','Ships scrapped'],['scrappedTonnage','Scrapped tonnage','kT']]},
  {title:'Economy',rows:[['resourcesProduced','Resources produced'],['productionResourcesSpent','Resources allocated to production'],['allocationRefundsResources','Production allocation refunds'],['researchResourcesSpent','Resources spent on research'],['ironiumSpent','Ironium spent','kT'],['boraniumSpent','Boranium spent','kT'],['germaniumSpent','Germanium spent','kT']]},
  {title:'Mining and salvage',rows:[['ironiumMined','Ironium mined','kT'],['boraniumMined','Boranium mined','kT'],['germaniumMined','Germanium mined','kT'],['mineralsSalvaged','Debris minerals recovered','kT']]}
 ];
 const format=(value?:bigint|number)=>new Intl.NumberFormat('en-US').format(value??0);
 function value(c:StatisticsCounters|undefined,key:keyof StatisticsCounters){const v=c?.[key];return typeof v==='bigint'?format(v):'0';}
 let history=$derived([...(stats?.accounting?.history??[])].reverse());
</script>

<div class="space-y-5">
 <p class="text-base-content/90">
  {#if stats?.accounting?.sinceYear}Detailed counters tracked from year <strong>{stats.accounting.sinceYear}</strong>. Older expenditure and kills are not reconstructed.
  {:else}Detailed tracking starts with the next generated turn. Historical expenditure and kills are not reconstructed.{/if}
 </p>
 <p>Previously recorded production: <strong>{format(Math.max(0,(stats?.tokensBuilt??0)-(stats?.starbasesBuilt??0)))}</strong> ships and <strong>{format(stats?.starbasesBuilt)}</strong> starbases. Colonies founded: <strong>{format(stats?.planetsColonized)}</strong>.</p>
 <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
  {#each groups as group}
   <section class="rounded border border-base-content/25 bg-base-200 p-4">
    <h2 class="font-bold text-lg mb-3">{group.title}</h2>
    <dl class="space-y-2">
     {#each group.rows as [key,label,unit]}
      <div class="flex justify-between gap-4"><dt>{label}</dt><dd class="font-mono font-bold text-right" data-stat={key}>{value(stats?.accounting?.counters,key)}{unit?' '+unit:''}</dd></div>
     {/each}
    </dl>
   </section>
  {/each}
 </div>
 <p class="text-sm text-base-content/90">Production spending records new allocations, including unfinished orders; refunds are listed separately. Tonnage uses ship design mass, without transported cargo. Destruction is credited to the firing player. Ship counts in battles include orbital bases.</p>
 <section class="overflow-x-auto rounded border border-base-content/25 bg-base-200 p-4">
  <h2 class="font-bold text-lg mb-3">Yearly history</h2>
  {#if history.length}
   <table class="table table-sm"><thead><tr><th>Year</th><th>Built</th><th>Destroyed</th><th>Lost</th><th>Production resources</th><th>Research resources</th><th>Minerals mined (kT)</th></tr></thead>
    <tbody>{#each history as row}<tr><td>{row.year}</td><td>{value(row.counters,'shipsBuilt')}</td><td class="text-success">{value(row.counters,'shipsDestroyed')}</td><td class="text-error">{value(row.counters,'shipsLost')}</td><td>{value(row.counters,'productionResourcesSpent')}</td><td>{value(row.counters,'researchResourcesSpent')}</td><td>{format((row.counters?.ironiumMined??0n)+(row.counters?.boraniumMined??0n)+(row.counters?.germaniumMined??0n))}</td></tr>{/each}</tbody>
   </table>
   <p class="text-sm mt-2">Up to 200 recent years are retained; lifetime counters continue accumulating.</p>
  {:else}<p>No completed year recorded yet.</p>{/if}
 </section>
</div>
