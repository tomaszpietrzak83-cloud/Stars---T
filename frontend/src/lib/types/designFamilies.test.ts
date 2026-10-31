import {create} from '@bufbuild/protobuf';
import {expect,it} from 'vitest';
import {ShipDesignSchema} from './cs-proto.js';
import {designFamilyMembers,latestFamilyDesign,upgradeDesignName} from './designFamilies.js';
it('keeps independent designs separate and finds the latest family version',()=>{
 const designs=[create(ShipDesignSchema,{num:1,name:'Sword',version:1}),create(ShipDesignSchema,{num:2,name:'Sword Mk 2',version:2}),create(ShipDesignSchema,{num:3,name:'Other',version:9})];
 const families={1:-1,2:-1};expect(designFamilyMembers(designs,families,1)).toHaveLength(2);expect(latestFamilyDesign(designs,families,1)?.num).toBe(2);expect(upgradeDesignName(designs[1],3)).toBe('Sword Mk 3');expect(designFamilyMembers(designs,families,3)).toHaveLength(1);
});
