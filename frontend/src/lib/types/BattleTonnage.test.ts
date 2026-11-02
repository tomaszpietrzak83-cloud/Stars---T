import {create} from '@bufbuild/protobuf';
import {expect,it} from 'vitest';
import {destroyedBattleTonnage} from './Battle.js';
import {BattleRecordSchema,BattleRecordTokenSchema,BattleRecordDestroyedTokenSchema,BattleRecordStatsSchema} from './cs-proto.js';
it('counts only destroyed ships from partial stacks, retaining both sides and allies',()=>{
 const record=create(BattleRecordSchema,{tokens:[create(BattleRecordTokenSchema,{num:1,playerNum:1,mass:40,startingQuantity:7}),create(BattleRecordTokenSchema,{num:2,playerNum:2,mass:120,startingQuantity:4})],destroyedTokens:[create(BattleRecordDestroyedTokenSchema,{num:1,playerNum:1,quantity:2}),create(BattleRecordDestroyedTokenSchema,{num:2,playerNum:2,quantity:3})],stats:create(BattleRecordStatsSchema,{shipsDestroyedByPlayer:{1:2,2:3}})});
 expect(destroyedBattleTonnage(record,new Set([1]),true)).toBe(80);
 expect(destroyedBattleTonnage(record,new Set([1]),false)).toBe(360);
 expect(destroyedBattleTonnage(record,new Set([1,2]),true)).toBe(440);
 record.tokens=[];expect(destroyedBattleTonnage(record,new Set([1]),true)).toBeUndefined();
});
