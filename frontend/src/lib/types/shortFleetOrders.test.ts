import {it,expect} from 'vitest';
import {create} from '@bufbuild/protobuf';
import {FleetSchema,MapObjectSchema,LogisticsAutomationSchema} from './cs-proto.js';
import {shortFleetOrders} from './shortFleetOrders.js';
it('shows manual, automatic and rebuilding orders without exposing enemy orders',()=>{
 const f=create(FleetSchema,{mapObject:create(MapObjectSchema,{num:1,tags:{}})});
 expect(shortFleetOrders(f)).toBe('MANUAL');
 const a=create(LogisticsAutomationSchema,{fleetRoles:{1:'patrol'}});expect(shortFleetOrders(f,a)).toBe('PATROL');
 f.mapObject!.tags['design-maintenance']='{}';expect(shortFleetOrders(f,a)).toBe('REBUILD');
});
