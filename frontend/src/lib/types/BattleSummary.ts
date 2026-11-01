import type { BattleRecord } from './cs-proto.js';
import type { DesignFinder } from '#lib/services/Universe.js';

// Unknown designs count conservatively: never hide a potentially major battle.
export function warshipsInBattle(battle: BattleRecord, finder: DesignFinder): number {
 return battle.tokens.reduce((total, token) => {
   const spec = finder.getDesign(token.playerNum, token.designNum)?.spec;
   return total + (!spec || spec.hasWeapons ? token.startingQuantity : 0);
 }, 0);
}

export function noteworthyBattle(battle: BattleRecord, finder: DesignFinder): boolean {
 return Object.values(battle.stats?.shipsDestroyedByPlayer ?? {}).some(n => n > 0)
   || warshipsInBattle(battle, finder) > 10;
}
