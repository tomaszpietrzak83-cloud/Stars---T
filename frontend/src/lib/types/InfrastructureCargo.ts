export function packedInstallationMass(object:{mapObject?:{tags?:Record<string,string>}}|undefined):number {
 try{const raw=object?.mapObject?.tags?.['infrastructure-cargo'];if(!raw)return 0;const c=JSON.parse(raw);const count=(v:unknown)=>typeof v==='number'&&Number.isFinite(v)?Math.max(0,v):0;return count(c.factories)*5+count(c.mines)*3+count(c.defenses);}catch{return 0;}
}
