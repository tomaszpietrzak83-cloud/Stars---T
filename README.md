<!-- Local edition: upstream attribution and licence are preserved. -->
# tomek-stars

A local edition built on **craig-stars**, created by **Craig (sirgwain)**: https://github.com/sirgwain/craig-stars.


## Play on another Windows computer

Download this repository with **Code → Download ZIP**, extract it completely, and double-click **tomek-stars.bat**. The Windows server and compiled interface are included. No Go, Zig, npm, Vite or account registration is needed to play. If a compatible Node.js is unavailable, the launcher downloads a portable LTS runtime from nodejs.org, checks its published SHA-256, and keeps it inside `.runtime` in this game folder.

Open the game at `http://localhost:5173`. The launcher signs into the local game automatically and restarts only processes belonging to this copy. A first start creates a fresh local database. Personal games are stored in `data/data.db`; **this folder is intentionally excluded from Git**. To move an existing game, stop both copies and separately copy the entire `data` folder. Keep a backup before replacing it. Updates replace program files, not `data`.

The source is Go and Svelte/TypeScript. Developers need the Go version from `go.mod`, Node.js 22.17 or newer, and a CGO-capable C compiler. Run `npm ci` in `frontend`; build `wasm/main.go` using `GOOS=js GOARCH=wasm CGO_ENABLED=0` into `frontend/src/lib/wasm/cs.wasm`; run `npm run build` in `frontend`; build the native server with CGO into `windows/stars-local.exe`. The committed edition can be played without rebuilding.

## Automation additions

- Fuel planning uses actual engine economy, payload, safe speeds, owned fuel docks and reserved finite tanker stocks. Routes stop and replan when a service becomes unavailable. Unknown enemies and future enemy orders cannot be predicted.
- Claim Adjuster colonizers evaluate instant terraforming automatically. Other races require **Serve worlds requiring terraforming** in Automation before currently red but recoverable worlds are selected. Ordinary minimum terraforming is then queued and paid for.
- The CA **Orbital Adjuster** terraforms from orbit. **Retro Bomb** worsens the environment. Normal planetary terraforming uses the production queue, not a transported building.
- **Carry surplus installations on existing routes** transports real factories, mines and defenses opportunistically. It never creates a special pickup trip. At current operating capacity 100, 95 remain: stock 110 can export 15. Packed factories weigh 5 kT, mines 3 kT and defenses 1 kT. Needed minerals fill spare room. Committed passenger deliveries can unload up to the peak-growth band to avoid carrying small leftovers repeatedly.
- The map displays the full fuel-budgeted scout petal, including planned stops and the return dock. Automatic transports also receive delivery and return waypoints. Navigation remains subject to safety checks each turn. Existing stranded ships use funded emergency returns and request nearby mobile fuel; ordinary new expeditions require an efficient return budget before departure. Returning scouts accept enemy-fleet risk when a safe route is unavailable or substantially slower, while retaining fuel, minefield and engine-safety checks.
- Patrols spread to frontier sectors at economical speed, combine against stronger targets and can refuel nearby automatic ships. Free escorts can also answer emergency fuel requests. They stage across frontier sectors at economical speed and can accept a convoy while travelling; impossible escort requirements release their reservations instead of blocking the whole escort pool. Mobile tankers retain actual fuel reserves, approach customers and provide non-stacking repairs. Bombers require escort or conservative cloak-adjusted early-warning coverage.
- Passenger transports spread among valuable worlds and depart only with a full useful load. The native population formula reaches maximum nominal births at one third of capacity; that is the default source reserve. Mineral deliveries can accelerate queued starbases before a dock exists. Priority planet markers coordinate colonization, bombardment, immediate landings and reinforcement.
- **defaultauto** recalculates balanced factory/mine development over a 27-year forecast. Temporary production boosters ahead of a starbase are retained only when the native simulation predicts earlier completion. Upgrading designs updates active fleet compositions to the newest version without restarting completed one-shot orders.
- New-game settings are remembered locally, including races, opponents and starting location. **Reset to defaults** restores defaults. The slider accepts 0–144 AI; the map must contain enough planets. **Edge/Center** select the human capital location while keeping normal opponent placement rules.
- After a human is publicly declared victor, AI prefer reachable attacks on that victor and prioritize military production while retaining territorial defense. No free resources or hidden intelligence are granted.

**EEE Bastion Station** is a late base for every race: six 20-weapon batteries, 1250 armor and an unlimited dock. **EEE Sanctuary Star**, restricted to Alternate Reality, retains four 32-weapon batteries and supports 3.75 million inhabitants with 1875 armor. Both use native-style station appearances, eight variants and symmetric slot layouts.

Developers can export navigation-only plans with `preview-automation --database SNAPSHOT.db --game ID --player NUM --output routes.json`. This command does not apply orders or generate a turn. Never save its scratch simulation as a live game.

The request inventory is `SUGGESTIONS-CHECKLIST.txt`. Upstream authorship and licence remain below.

## Origin and contributions

This project is an open-source derivative of Craig (sirgwain)'s open-source **craig-stars**: https://github.com/sirgwain/craig-stars. Craig's simulation and client form the foundation. Tomek-stars adds single-player automation, fuel logistics, fleet composition and replacement, exploration, expanded maps, race options and technologies. Both projects remain open source under the repository licence. Original Stars! was created by Jeff Johnson and Jeff McBride.

Contributions should include a clear behavior description and relevant tests. Never publish personal saves. Our release series starts at **0.2.0** for the automation edition; increment patch versions for fixes and minor versions for new gameplay capabilities. See SUGGESTIONS-CHECKLIST.txt for implementation status, including unfinished requests.
