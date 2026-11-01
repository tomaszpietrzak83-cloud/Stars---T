import { create } from '@bufbuild/protobuf';
import { expect, it } from 'vitest';
import { BattleRecordSchema, BattleRecordStatsSchema, BattleRecordTokenSchema, ShipDesignSchema, ShipDesignSpecSchema } from './cs-proto.js';
import { noteworthyBattle } from './BattleSummary.js';

const finder = { getDesign: (_p: number, n: number) => n === 3 ? undefined : create(ShipDesignSchema, { spec: create(ShipDesignSpecSchema, { hasWeapons: n === 1 }) }), getMyDesign: () => undefined };
it('retains losses and battles with more than ten warships, aggregating minor no-loss encounters', () => {
 const make = (designNum: number, count: number, losses = 0) => create(BattleRecordSchema, {
   tokens: [create(BattleRecordTokenSchema, { designNum, startingQuantity: count })],
   stats: create(BattleRecordStatsSchema, { shipsDestroyedByPlayer: { 2: losses } })
 });
 expect(noteworthyBattle(make(1, 10), finder)).toBe(false);
 expect(noteworthyBattle(make(1, 11), finder)).toBe(true);
 expect(noteworthyBattle(make(2, 100), finder)).toBe(false);
 expect(noteworthyBattle(make(2, 1, 1), finder)).toBe(true);
 expect(noteworthyBattle(make(3, 11), finder)).toBe(true);
});
