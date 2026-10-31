import type {ShipDesign} from './cs-proto.js';
export function designFamilyMembers(designs:ShipDesign[],families:Record<number,number>,num:number){const id=families[num];return id?designs.filter(d=>families[d.num]===id):designs.filter(d=>d.num===num);}
export function latestFamilyDesign(designs:ShipDesign[],families:Record<number,number>,num:number){return designFamilyMembers(designs,families,num).sort((a,b)=>b.version-a.version||b.num-a.num)[0];}
export function upgradeDesignName(design:ShipDesign,version:number){return `${design.name.replace(/\s+Mk\s+\d+$/i,'')} Mk ${version}`;}
