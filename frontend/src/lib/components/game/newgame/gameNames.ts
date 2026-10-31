const places=['Orion','Vega','Andromeda','Cygnus','Antares','Perseus','Sirius','Cassiopeia','Centauri','Altair','Lyra','Draco','Arcturus','Deneb','Carina','Eridanus'];
const events=['Echoes','Embers','Shadows','Guardians','Voyagers','Relics','Dreams','Exiles','Legends','Signals','Children','Ghosts','Wanderers','Secrets','Banners','Oaths'];
const adjectives=['Silent','Distant','Forgotten','Uncharted','Crimson','Shattered','Last','Hidden','Eternal','Starlit','Lost','Golden','Endless','Sleeping','Broken','Rising'];
const frontiers=['Frontier','Horizon','Empire','Constellation','Expanse','Meridian','Nebula','Alliance','Passage','Dominion','Odyssey','Beacon','Sanctuary','Voyage','Covenant','Dawn'];

export function randomGameName(previous='', random:()=>number=Math.random):string {
	const pick=(items:string[])=>items[Math.min(items.length-1,Math.floor(random()*items.length))];
	for(let attempt=0;attempt<12;attempt++) {
		const form=Math.floor(random()*4);
		const name=form===0?`${pick(events)} of ${pick(places)}`:form===1?`The ${pick(adjectives)} ${pick(frontiers)}`:form===2?`${pick(events)} Beyond ${pick(places)}`:`${pick(places)}: ${pick(adjectives)} ${pick(frontiers)}`;
		if(name!==previous) return name;
	}
	return previous==='The Uncharted Horizon'?'Echoes of Orion':'The Uncharted Horizon';
}
