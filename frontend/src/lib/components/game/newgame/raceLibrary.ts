import { raceClient } from '#lib/services/connect.js';
import type { Race } from '#lib/types/cs-proto.js';

let request:Promise<Race[]>|undefined;
let loadedAt=0;
// A 64-player setup shares one request, rather than loading the same library
// once per opponent. Short expiry lets recently saved custom races appear.
export function loadRaceLibrary():Promise<Race[]> {
	if(!request || Date.now()-loadedAt>10000) {
		loadedAt=Date.now();
		request=raceClient.getRaces({}).then(response=>response.races).catch(error=>{request=undefined;throw error;});
	}
	return request;
}
