import { create } from '@bufbuild/protobuf';
import { RaceSchema, TechField } from '#lib/types/cs-proto.js';
import { IFE, ARM, UR } from '#lib/types/Race.js';
import { expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import EEETraits from './EEETraits.svelte';

it('offers all sixteen EEE traits and replaces opposing selections', async()=>{
	const race=create(RaceSchema);
	const {container}=render(EEETraits,{race});
	expect(container.querySelectorAll('input[type="checkbox"]')).toHaveLength(16);
	await page.getByLabelText('EEE — Efficient Drives').click();
	await page.getByLabelText('EEE — Inefficient Propulsion').click();
	await expect.element(page.getByLabelText('EEE — Efficient Drives')).not.toBeChecked();
	await expect.element(page.getByLabelText('EEE — Inefficient Propulsion')).toBeChecked();
});

it('shows IFE ARM UR prices and allows a research field', async()=>{
	const race=create(RaceSchema,{lrts:IFE|ARM|UR});
	const {container}=render(EEETraits,{race});
	expect(container.querySelector('[aria-label="EEE — Extraction Specialists"]')?.closest('label')?.textContent).toContain('60 pts');
	expect(container.querySelector('[aria-label="EEE — Material Recovery"]')?.closest('label')?.textContent).toContain('30 pts');
	await page.getByLabelText('EEE — Research Specialization',{exact:true}).click();
	await page.getByRole('combobox',{name:'EEE — Specialization field'}).selectOptions(String(TechField.ELECTRONICS));
	await expect.element(page.getByRole('combobox',{name:'EEE — Specialization field'})).toHaveValue(String(TechField.ELECTRONICS));
	await page.getByLabelText('EEE — Research Specialization',{exact:true}).click();
	await expect.element(page.getByRole('combobox',{name:'EEE — Specialization field'})).not.toBeInTheDocument();
});
