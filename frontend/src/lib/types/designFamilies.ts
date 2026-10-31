import type {ShipDesign} from './cs-proto.js';
export function designFamilyMembers(designs:ShipDesign[],families:Record<number,number>,num:number){const id=families[num];return id?designs.filter(d=>families[d.num]===id):designs.filter(d=>d.num===num);}
export function latestFamilyDesign(designs:ShipDesign[],families:Record<number,number>,num:number){return designFamilyMembers(designs,families,num).sort((a,b)=>b.version-a.version||b.num-a.num)[0];}
export function upgradeDesignName(design:ShipDesign,version:number){return `${design.name.replace(/\s+Mk\s+\d+$/i,'')} Mk ${version}`;}

// Keep referenced blueprints available to queues and reports, but retire empty
// older versions from the designer as soon as their last ship is gone.
export function hideSupersededDesign(design: ShipDesign, designs: ShipDesign[], families: Record<number, number>, maintenance: Record<number, number>, queued: number): boolean {
 if (design.cannotDelete || design.spec?.numInstances || queued > 0) return false;
 const successor = designs.find(d => d.num === maintenance[design.num]);
 const latest = latestFamilyDesign(designs, families, design.num);
 return !!(successor && successor.num !== design.num) || !!(latest && latest.num !== design.num && latest.version > design.version);
}
