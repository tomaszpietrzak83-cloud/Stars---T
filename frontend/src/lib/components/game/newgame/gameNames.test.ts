import {expect,it} from 'vitest';
import {randomGameName} from './gameNames';
it('produces readable varied names and avoids immediately repeating a name',()=>{
	const names=new Set(Array.from({length:150},()=>randomGameName()));
	expect(names.size).toBeGreaterThan(100);
	for(const name of names){expect(name.length).toBeGreaterThan(8);expect(name.length).toBeLessThan(50);expect(name).not.toMatch(/undefined|\d/);}
	const previous=randomGameName('',()=>0);
	expect(randomGameName(previous,()=>0)).not.toBe(previous);
});
