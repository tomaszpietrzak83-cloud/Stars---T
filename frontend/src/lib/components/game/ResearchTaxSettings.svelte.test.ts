import {create} from '@bufbuild/protobuf';
import {expect,it,vi} from 'vitest';
import {page} from 'vitest/browser';
import {render} from 'vitest-browser-svelte';
import {LogisticsAutomationSchema} from '#lib/types/cs-proto.js';
import ResearchTaxSettings from './ResearchTaxSettings.svelte';
it('saves resource-based thresholds and separate rates including zero',async()=>{
 const automation=create(LogisticsAutomationSchema);
 const onSave=vi.fn(async(change:(value:typeof automation)=>void)=>{change(automation);});
 render(ResearchTaxSettings,{automation,onSave});
 await page.getByRole('checkbox',{name:'Enable automatic colony research tax'}).click();
 await page.getByRole('combobox',{name:'Research tax size basis'}).selectOptions('resources');
 await page.getByRole('spinbutton',{name:'Small colony research (%)'}).fill('0');
 await page.getByRole('spinbutton',{name:'Medium colony research (%)'}).fill('15');
 await page.getByRole('button',{name:'Save colony research tax'}).click();
 expect(automation.researchTax).toMatchObject({enabled:true,basis:'resources',leftoverUntil:50,mediumThreshold:250,largeThreshold:1000,smallPercent:0,mediumPercent:15,largePercent:30});
});
it('blocks inverted thresholds',async()=>{
 render(ResearchTaxSettings,{automation:create(LogisticsAutomationSchema),onSave:vi.fn(async()=>{})});
 await page.getByRole('spinbutton',{name:'Research tax medium threshold'}).fill('900000');
 await expect.element(page.getByRole('button',{name:'Save colony research tax'})).toBeDisabled();
});
