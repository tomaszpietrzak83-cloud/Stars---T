import { create } from '@bufbuild/protobuf';
import { FleetSchema, WaypointTask } from '#lib/types/cs-proto.js';
import { canPerformWaypointTask } from './Fleet';
import { expect,it } from 'vitest';

it('hides colony minefield and mining orders from unequipped fleets',()=>{
	const fleet=create(FleetSchema,{spec:{shipDesignSpec:{cargoCapacity:100,fuelCapacity:100}}});
	for(const task of [WaypointTask.COLONIZE,WaypointTask.REMOTE_MINING,WaypointTask.LAY_MINEFIELD]) expect(canPerformWaypointTask(fleet,task)).toBe(false);
	fleet.spec!.shipDesignSpec!.miningRate=10;
	expect(canPerformWaypointTask(fleet,WaypointTask.REMOTE_MINING)).toBe(true);
	expect(canPerformWaypointTask(fleet,WaypointTask.COLONIZE)).toBe(false);
	fleet.spec!.shipDesignSpec!.colonizer=true;fleet.spec!.shipDesignSpec!.canLayMines=true;
	expect(canPerformWaypointTask(fleet,WaypointTask.COLONIZE)).toBe(true);
	expect(canPerformWaypointTask(fleet,WaypointTask.LAY_MINEFIELD)).toBe(true);
});
