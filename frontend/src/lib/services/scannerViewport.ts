export type ScannerViewport = { x:number; y:number; scale:number };
const viewports=new Map<string,ScannerViewport>();
const valid=(v:ScannerViewport|undefined):v is ScannerViewport=>!!v && [v.x,v.y,v.scale].every(Number.isFinite) && v.scale>0;
const storageKey=(key:string)=>`tomek-stars:viewport:${key}`;
export function rememberScannerViewport(key:string,view:ScannerViewport) {
 if(!valid(view))return;viewports.set(key,{...view});
 try{if(typeof sessionStorage!=='undefined')sessionStorage.setItem(storageKey(key),JSON.stringify(view));}catch{/* storage may be unavailable */}
}
export function recalledScannerViewport(key:string):ScannerViewport|undefined {
 const cached=viewports.get(key);if(cached)return {...cached};
 try{if(typeof sessionStorage!=='undefined'){const saved=JSON.parse(sessionStorage.getItem(storageKey(key))??'null');if(valid(saved)){viewports.set(key,saved);return {...saved};}}}catch{/* ignore malformed old data */}
 return undefined;
}
export function forgetScannerViewport(key:string){viewports.delete(key);try{if(typeof sessionStorage!=='undefined')sessionStorage.removeItem(storageKey(key));}catch{/* storage may be unavailable */}}
