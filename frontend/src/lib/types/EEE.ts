import { TechField, type Race } from '#lib/types/cs-proto.js';
import { IFE, ARM, UR } from './Race';

export const eeeTraits = [
	['Efficient Drives','Fuel use −10%; multiplies the IFE effect.',35],
	['Extended Tanks','Fuel capacity +20%.',25],
	['Cargo Engineering','Mineral cargo capacity +20%; passenger capacity is unchanged.',40],
	['Extraction Specialists','Remote mining +25%; also applies to ARM robots.',45],
	['Material Recovery','Planetary scrapping recovers +5 percentage points of minerals, capped at 95%.',20],
	['Maintenance Crews','Existing ship and starbase repair rates +25%.',30],
	['Field Workshops','Repairs an additional 1% of armor per turn while stationary without a starbase.',45],
	['Orbital Assembly','Ship resource cost −10%; mineral costs are unchanged.',70],
	['Station Engineers','Starbase construction and upgrade resource cost −10%; mineral costs are unchanged.',30],
	['Survey Networks','Scanner ranges +15%.',55],
	['Mine Navigation','Safe minefield speed +1 warp; does not protect against detonations.',45],
	['Research Specialization','Research cost −10% in one selected field.',45],
	['Inefficient Propulsion','Fuel use +15%.',-25],
	['Lightweight Hulls','Ship hull armor −10%; installed armor plates are unchanged.',-35],
	['Expensive Shipbuilding','Ship resource cost +10%; mineral costs are unchanged.',-40],
	['Slow Maintenance','Existing ship and starbase repair rates −25%.',-20]
].map(([name,description,cost], index)=>({name:String(name),description:String(description),cost:Number(cost),flag:1<<index}));

export const eeeOpposites = [[0,12],[5,15],[6,15],[7,14]].map(([a,b])=>[1<<a,1<<b]);

export function eeeTraitCost(race: Race, flag: number): number {
	let cost=eeeTraits.find(trait=>trait.flag===flag)?.cost ?? 0;
	if ((race.lrts&IFE) && (flag===1 || flag===(1<<12))) cost+=10;
	if ((race.lrts&ARM) && flag===(1<<3)) cost+=15;
	if ((race.lrts&UR) && flag===(1<<4)) cost+=10;
	return cost;
}

export function selectEEETrait(race: Race, flag: number, selected: boolean) {
	if (selected) {
		for (const [a,b] of eeeOpposites) {
			if (a===flag) race.eeeTraits &= ~b;
			if (b===flag) race.eeeTraits &= ~a;
		}
		race.eeeTraits |= flag;
	} else race.eeeTraits &= ~flag;
	if (!(race.eeeTraits&(1<<11))) race.eeeResearchField=TechField.UNSPECIFIED;
	else if (race.eeeResearchField===TechField.UNSPECIFIED) race.eeeResearchField=TechField.ENERGY;
}
