import { expect, it } from 'vitest';
import { PlayerSettings } from './PlayerSettings';
import { PlayerMessageType as Type } from './cs-proto';

it('filters upgrades independently from construction and general information after reload', () => {
	const settings = new PlayerSettings();
	settings.filterMessageType(Type.FLEET_BUILT);
	settings.filterMessageType(Type.INFO);
	settings.beforeSave();
	const restored = new PlayerSettings();
	restored.messageTypeFilterArray = [...settings.messageTypeFilterArray];
	restored.afterLoad();
	expect(restored.isMessageVisible(Type.FLEET_BUILT)).toBe(false);
	expect(restored.isMessageVisible(Type.INFO)).toBe(false);
	expect(restored.isMessageVisible(Type.SHIP_UPGRADES)).toBe(true);
	restored.filterMessageType(Type.SHIP_UPGRADES);
	restored.showMessageType(Type.FLEET_BUILT);
	expect(restored.isMessageVisible(Type.SHIP_UPGRADES)).toBe(false);
	expect(restored.isMessageVisible(Type.FLEET_BUILT)).toBe(true);
});
