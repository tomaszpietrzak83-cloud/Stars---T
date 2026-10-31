<script lang="ts">
	import NewGame from '#lib/components/game/newgame/NewGame.svelte';
	import { getColor } from '#lib/components/game/newgame/playerColors.js';
	import {
		AiDifficulty,
		NewGamePlayerSchema,
		NewGamePlayerType,
		type NewGamePlayer
	} from '#lib/types/cs-proto.js';
	import { create } from '@bufbuild/protobuf';

	const players: NewGamePlayer[] = [
		create(NewGamePlayerSchema, {
			type: NewGamePlayerType.HOST,
			color: getColor(0),
			aiDifficulty: AiDifficulty.UNSPECIFIED
		}),
		create(NewGamePlayerSchema, {
			type: NewGamePlayerType.OPEN,
			color: getColor(1),
			aiDifficulty: AiDifficulty.NORMAL
		}),
		create(NewGamePlayerSchema, {
			type: NewGamePlayerType.AI,
			color: getColor(2),
			aiDifficulty: AiDifficulty.NORMAL
		})
	];

</script>

<NewGame {players} />
