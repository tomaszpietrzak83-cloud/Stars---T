import { create } from '@bufbuild/protobuf';
import { RaceSchema,Size,Density,AiDifficulty } from '#lib/types/cs-proto.js';
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
