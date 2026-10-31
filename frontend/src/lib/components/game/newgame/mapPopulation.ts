import { Size, Density } from '#lib/types/cs-proto.js';

export function mapDimensions(size:Size):[number,number]|undefined {
	return ({[Size.TINY]:[400,400],[Size.TINY_WIDE]:[500,300],[Size.SMALL]:[800,800],[Size.SMALL_WIDE]:[1000,600],[Size.MEDIUM]:[1200,1200],[Size.MEDIUM_WIDE]:[1500,900],[Size.LARGE]:[1600,1600],[Size.LARGE_WIDE]:[2000,1200],[Size.HUGE]:[2000,2000],[Size.HUGE_WIDE]:[2500,1500],[Size.VAST]:[2800,2800],[Size.IMMENSE]:[3200,3200],[Size.COLOSSAL]:[4000,4000],[Size.TITANIC]:[5000,5000],[Size.GALACTIC]:[6000,6000]} as Partial<Record<Size,[number,number]>>)[size];
}

export function estimatedStars(size:Size,density:Density,sqLyPerPlanet=5000):number|undefined {
	const dimensions=mapDimensions(size);
	if(!dimensions || sqLyPerPlanet<=0) return undefined;
	const base=Math.floor(dimensions[0]*dimensions[1]/sqLyPerPlanet);
	switch(density) {
		case Density.SPARSE:return base-Math.floor(base/4);
		case Density.NORMAL:return base;
		case Density.DENSE:return base+Math.floor(base/4);
		case Density.PACKED:return base+Math.floor(base*3/4);
		default:return undefined;
	}
}

// Rounded from 16 empires per Huge map's 4 million square light years.
export function recommendedEmpires(size: Size): number | undefined {
	switch (size) {
		case Size.VAST: return 32;
		case Size.IMMENSE: return 40;
		case Size.COLOSSAL: return 64;
		case Size.TITANIC: return 100;
		case Size.GALACTIC: return 144;
		default: return undefined;
	}
}

