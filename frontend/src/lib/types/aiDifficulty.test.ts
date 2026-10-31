import {expect,it} from 'vitest';
import {AiDifficulty} from './cs-proto.js';
import {difficultyLevels,variedDifficulty} from './aiDifficulty.js';
it('uses seven ordered levels and a declining symmetric distribution',()=>{
 expect(difficultyLevels).toHaveLength(7);
 const counts=new Map<number,number>();
 for(let i=0;i<10000;i++){const value=variedDifficulty(AiDifficulty.ADVANCED,()=> (i+.5)/10000);counts.set(value,(counts.get(value)??0)+1);}
 const n=difficultyLevels.map(l=>counts.get(l.value)??0);
 expect(n[3]).toBeGreaterThan(n[2]);expect(n[2]).toBeGreaterThan(n[1]);expect(n[1]).toBeGreaterThan(n[0]);
 expect(n[0]).toBe(n[6]);expect(n[1]).toBe(n[5]);expect(n[2]).toBe(n[4]);
});
it('excludes out-of-range outcomes without clamping and never exceeds four steps',()=>{
 for(let b=0;b<7;b++)for(let i=0;i<1000;i++){
  const value=variedDifficulty(difficultyLevels[b].value,()=> (i+.5)/1000);
  const index=difficultyLevels.findIndex(l=>l.value===value);
  expect(index).toBeGreaterThanOrEqual(0);expect(Math.abs(index-b)).toBeLessThanOrEqual(4);
 }
});
