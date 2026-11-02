import { create, fromJsonString, toJsonString } from '@bufbuild/protobuf';
import { RaceSchema,Size,Density,AiDifficulty,GameSettingsSchema,NewGamePlayerType } from '#lib/types/cs-proto.js';
import { expect,it,vi,beforeEach } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import NewGame from './NewGame.svelte';
import { gameClient } from '#lib/services/connect.js';

vi.mock('#lib/services/connect.js',async(importOriginal)=>({
	...await importOriginal<typeof import('#lib/services/connect.js')>(),
	raceClient:{getRaces:vi.fn(async()=>({races:[create(RaceSchema,{id:-1n,name:'Humanoid',pluralName:'Humanoids'})]}))},
	gameClient:{createGame:vi.fn(async()=>({game:{game:{id:1n}}}))}
}));
vi.mock('$app/navigation',()=>({goto:vi.fn(async()=>{})}));
beforeEach(()=>{localStorage.clear();vi.clearAllMocks();vi.stubGlobal('fetch',vi.fn(async()=>({headers:new Headers({'X-App-Capabilities':'expanded-maps'})})));});

it('updates star estimates, randomizes names and applies difficulty to every AI',async()=>{
	render(NewGame);
	const name=page.getByRole('textbox',{name:'Name',exact:true});
	const previous=(name.element() as HTMLInputElement).value;
	await page.getByRole('button',{name:'Randomize game name'}).click();
	expect((name.element() as HTMLInputElement).value).not.toBe(previous);
	await page.getByRole('combobox',{name:'Size',exact:true}).selectOptions(String(Size.GALACTIC));
	await page.getByRole('combobox',{name:'Density',exact:true}).selectOptions(String(Density.PACKED));
	await expect.element(page.getByLabelText('Estimated star count')).toHaveTextContent(/12[\s,]*600/);
	await page.getByRole('combobox',{name:'Difficulty for all AI'}).selectOptions(String(AiDifficulty.HARD));
	await page.getByRole('button',{name:'Apply to all AI'}).click();
	await page.getByRole('button',{name:'Create Game',exact:true}).click();
	const settings=vi.mocked(gameClient.createGame).mock.calls[0][0].settings!;
	expect(settings.players!.slice(1).every(player=>player.aiDifficulty===AiDifficulty.HARD)).toBe(true);
});

it('shows creation errors instead of silently staying on the form',async()=>{
	vi.mocked(gameClient.createGame).mockRejectedValueOnce(new Error('Cannot create this galaxy'));
	render(NewGame);
	await page.getByRole('button',{name:'Create Game',exact:true}).click();
	await expect.element(page.getByRole('alert')).toHaveTextContent('Cannot create this galaxy');
});

it('explains difficulty effects before a game is created', async()=>{
	render(NewGame);
	await page.getByText('How AI difficulty works', {exact:true}).click();
	await expect.element(page.getByText(/does not create free resources/)).toBeVisible();
});

it('supports exact AI counts, persists settings and resets to defaults',async()=>{
 const first=render(NewGame);
 await page.getByRole('slider',{name:'AI opponents'}).fill('7');
 await page.getByRole('combobox',{name:'Starting location'}).selectOptions('edge');
 await page.getByRole('combobox',{name:'Size',exact:true}).selectOptions(String(Size.GALACTIC));
 await expect.poll(()=>localStorage.getItem('tomek-stars.new-game.v1')).toContain('edge');
 await first.unmount();
 render(NewGame);
 await expect.element(page.getByRole('slider',{name:'AI opponents'})).toHaveValue('7');
 await expect.element(page.getByRole('combobox',{name:'Starting location'})).toHaveValue('edge');
 await page.getByRole('button',{name:'Reset to defaults'}).click();
 await expect.element(page.getByRole('slider',{name:'AI opponents'})).toHaveValue('2');
 await expect.element(page.getByRole('combobox',{name:'Starting location'})).toHaveValue('');
 await page.getByRole('slider',{name:'AI opponents'}).fill('144');
 await expect.element(page.getByRole('slider',{name:'AI opponents'})).toHaveValue('144');
 await page.getByRole('slider',{name:'AI opponents'}).fill('0');
 await expect.element(page.getByRole('slider',{name:'AI opponents'})).toHaveValue('0');
});

it('restores victory criteria and values with map options, submits them and resets them together', async () => {
	const first = render(NewGame);
	const ownPlanets = page.getByRole('spinbutton').nth(0);
	const years = page.getByRole('spinbutton').last();
	await ownPlanets.fill('80');
	await years.fill('120');
	await page.getByRole('checkbox').last().click();
	await page.getByRole('combobox', { name: 'Density', exact: true }).selectOptions(String(Density.PACKED));
	await expect.poll(() => fromJsonString(GameSettingsSchema, localStorage.getItem('tomek-stars.new-game.v1')!).victoryConditions?.yearsPassed).toBe(120);
	const saved = fromJsonString(GameSettingsSchema, localStorage.getItem('tomek-stars.new-game.v1')!);
	await first.unmount();
	render(NewGame);
	await expect.element(page.getByRole('spinbutton').nth(0)).toHaveValue(80);
	await expect.element(page.getByRole('spinbutton').last()).toHaveValue(120);
	await expect.element(page.getByRole('checkbox').last()).toBeChecked();
	await expect.element(page.getByRole('combobox', { name: 'Density', exact: true })).toHaveValue(String(Density.PACKED));
	await page.getByRole('button', { name: 'Create Game', exact: true }).click();
	const submitted = vi.mocked(gameClient.createGame).mock.calls[0][0].settings!;
	expect(submitted.victoryConditions).toEqual(saved.victoryConditions);
	await page.getByRole('button', { name: 'Reset to defaults' }).click();
	await expect.element(page.getByRole('spinbutton').nth(0)).toHaveValue(60);
	await expect.element(page.getByRole('spinbutton').last()).toHaveValue(50);
	await expect.element(page.getByRole('checkbox').last()).not.toBeChecked();
});

it('displays restored host and AI races even when they are absent from the current library', async () => {
	const race = create(RaceSchema, { id: 42n, name: 'Saved race', pluralName: 'Saved people' });
	const saved = create(GameSettingsSchema, { players: [
		{ type: NewGamePlayerType.HOST, race },
		{ type: NewGamePlayerType.AI, race, aiDifficulty: AiDifficulty.HARD }
	] });
	localStorage.setItem('tomek-stars.new-game.v1', toJsonString(GameSettingsSchema, saved));
	render(NewGame);
	await expect.element(page.getByRole('combobox', { name: 'Host', exact: true })).toHaveValue('42');
	await expect.element(page.getByRole('combobox', { name: 'AI race', exact: true })).toHaveValue('42');
	await expect.element(page.getByRole('combobox', { name: 'Difficulty', exact: true })).toHaveValue(String(AiDifficulty.HARD));
});
