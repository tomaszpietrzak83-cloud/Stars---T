import type { Fleet, ShipDesign, ShipDesignSpec } from './cs-proto.js';
export function dedicatedMinesweeper(spec?: ShipDesignSpec) {
	return (
		!!spec &&
		!spec.starbase &&
		spec.mineSweep > 0 &&
		spec.weaponSlots.length > 0 &&
		!spec.bombs.length &&
		!spec.smartBombs.length &&
		!spec.retroBombs.length &&
		spec.weaponSlots.every((s) => s.quantity > 0 && s.hullComponent === 'Gatling Gun')
	);
}
export function designAutoRole(spec?: ShipDesignSpec): string {
	if (spec?.defaultAutoRole === 'manual') return '';
	return dedicatedMinesweeper(spec) ? 'minesweeper' : (spec?.defaultAutoRole ?? '');
}
export function automationRoles(fleet: Fleet, designs: ShipDesign[] = [], currentRole = '') {
	const spec = fleet.spec?.shipDesignSpec;
	const roles = [{ value: '', label: 'Manual' }];
	if (
		dedicatedMinesweeper(spec) ||
		fleet.tokens.some((t) => dedicatedMinesweeper(designs.find((d) => d.num === t.designNum)?.spec))
	) {
		return [
			...roles,
			{ value: 'minesweeper', label: 'Minesweeper: clear hostile mines; avoid space combat' }
		];
	}
	if ((spec?.mineSweep ?? 0) > 0)
		roles.push({
			value: 'minesweeper',
			label: 'Minesweeper: clear hostile mines; avoid space combat'
		});
	if (spec?.scanner || !spec?.hasWeapons)
		roles.push({ value: 'scout', label: 'Scout: exploration and incoming fleet reconnaissance' });
	if (
		spec?.colonizer &&
		fleet.tokens.length &&
		fleet.tokens.every((t) => designs.find((d) => d.num === t.designNum)?.spec?.colonizer)
	) {
		for (const [value, label] of [
			['auto', 'Auto'],
			['nearest', 'Nearest'],
			['farthest', 'Farthest'],
			['best', 'Best world']
		])
			roles.push({ value: `colonizer:${value}`, label: `Colonizer: ${label}` });
	}
	if ((spec?.miningRate ?? 0) > 0) roles.push({ value: 'miner', label: 'Automatic remote mining' });
	if ((spec?.terraformRate ?? 0) > 0)
		roles.push({ value: 'terraformer', label: 'Automatic terraforming' });
	if ((spec?.cargoCapacity ?? 0) > 0)
		roles.push({ value: 'freighter', label: 'Automatic transport' });
	if (spec?.bomber)
		roles.push({ value: 'bomber', label: 'Bomber: autonomous raids and wolf packs' });
	if (spec?.hasWeapons) {
		roles.push({ value: 'patrol', label: 'Auto Patrol: intercept and regroup' });
		roles.push({ value: 'escort', label: 'Escort' });
		roles.push({ value: 'combat', label: 'Combat: manual mission with automatic safety' });
	}
	if ((spec?.fuelCapacity ?? 0) > 0)
		roles.push({ value: 'tanker', label: 'Tanker: automatic fuel supply' });
	if (currentRole && !roles.some((option) => option.value === currentRole))
		roles.push({
			value: currentRole,
			label: `${currentRole}: enabled, waiting for suitable equipment`
		});
	return roles;
}
export function automationStatus(fleet: Fleet): string {
	const tags = fleet.mapObject?.tags ?? {};
	if (tags['automatic-fuel-status']) return tags['automatic-fuel-status'];
	for (const key of [
		'automatic-trader',
		'automatic-patrol',
		'automatic-tanker',
		'design-maintenance',
		'logistics-job',
		'mining-job',
		'colonization-job',
		'terraform-job',
		'automatic-safety',
		'bomber-orders'
	]) {
		try {
			if (tags[key]) return JSON.parse(tags[key]).status ?? 'Order pending';
		} catch {
			/* legacy status */
		}
	}
	return (
		tags['scout-job'] ??
		(tags['escort-for']
			? `Escorting fleet #${tags['escort-for']}`
			: 'Orders take effect on the next turn')
	);
}

export function automationPlan(fleet: Fleet): string {
	const status = automationStatus(fleet).replace(/\s+/g, ' ').trim();
	const next = fleet.fleetOrders?.waypoints?.[1];
	if (!next) return status;
	const name =
		next.mapObjectTarget?.targetName ||
		(next.position ? `(${next.position.x}, ${next.position.y})` : 'the next waypoint');
	return `${status.replace(/[.!?]$/, '')}. Next: ${name}, warp ${next.warpSpeed}; fuel is checked before departure.`;
}
