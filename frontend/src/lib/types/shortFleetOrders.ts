import type { Fleet, LogisticsAutomation } from './cs-proto.js';
export function shortFleetOrders(f: Fleet, a?: LogisticsAutomation): string {
	const tags = f.mapObject?.tags ?? {};
	if (tags['design-maintenance']) return 'REBUILD';
	const role = a?.fleetRoles[f.mapObject?.num ?? 0];
	if (role)
		try {
			const job = JSON.parse(tags['automatic-patrol'] || 'null') as { status?: string } | null;
			if (job?.status?.includes('Gathering') || job?.status?.includes('Mobilizing'))
				return 'PATROL: GATHER';
			if (job?.status?.includes('Intercepting')) return 'PATROL: HUNT';
			const transport = JSON.parse(tags['logistics-job'] || 'null') as { kind?: string } | null;
			if (transport?.kind)
				return (
					(
						{
							planet: 'TRANSPORT: LOAD',
							passengers: 'TRANSPORT: LOAD',
							settle: 'TRANSPORT: PAX',
							return: 'TRANSPORT: MIN',
							'invade-load': 'TRANSPORT: LOAD',
							invade: 'TRANSPORT: LAND',
							salvage: 'TRANSPORT: DEBRIS'
						} as Record<string, string>
					)[transport.kind] || 'TRANSPORT'
				);
		} catch {
			/* malformed legacy job falls back to the configured role */
		}
	if (role)
		return (
			(
				{
					freighter: 'TRANSPORT',
					scout: 'SCOUT',
					patrol: 'PATROL',
					escort: 'ESCORT',
					tanker: 'TANKER',
					bomber: 'BOMB',
					miner: 'MINE',
					minesweeper: 'SWEEP',
					terraformer: 'TERRA'
				} as Record<string, string>
			)[role] ?? (role.startsWith('colonizer:') ? 'COLONIZE' : role.toUpperCase())
		);
	const wp = f.fleetOrders?.waypoints?.[1];
	return wp ? `MANUAL → ${wp.mapObjectTarget?.targetName || 'move'}` : 'MANUAL';
}
