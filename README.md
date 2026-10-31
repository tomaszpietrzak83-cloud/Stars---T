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
- Passenger transports spread among valuable worlds and depart only with a full useful load. The native population formula reaches maximum nominal births at one third of capacity; that is the default source reserve. Demand-derived mineral stocks cover the next committed unit and roughly three years of manufacturing; useful deliveries support ships and bases, including a base before its dock exists. Priority planet markers coordinate colonization, bombardment, immediate landings and reinforcement.
- **defaultauto** recalculates balanced factory/mine development over a 27-year forecast. Adding ships or a base immediately evaluates temporary factories and mines before manufacturing. Investments may accelerate completion or build useful infrastructure without delaying any explicit manufacturing entry. Fleet-composition orders use the same planner. Temporary alchemy requires an actual speed gain; temporary boosters disappear after manufacturing and existing defaultauto resumes. Forecasts use current known conditions, not future attacks or deliveries. Upgrading designs updates active fleet compositions to the newest version without restarting completed one-shot orders.
- New-game settings are remembered locally, including races, opponents and starting location. **Reset to defaults** restores defaults. The slider accepts 0–144 AI; the map must contain enough planets. **Edge/Center** select the human capital location while keeping normal opponent placement rules.
- After a human is publicly declared victor, AI prefer reachable attacks on that victor and prioritize military production while retaining territorial defense. No free resources or hidden intelligence are granted.

**EEE Bastion Station** is a late base for every race: six 20-weapon batteries, 1250 armor and an unlimited dock. **EEE Sanctuary Star**, restricted to Alternate Reality, has four 40-weapon batteries, six electrical banks of six and three orbital slots, and supports 3.75 million inhabitants with 1875 armor. Both have original station artwork, eight palette variants and symmetric slot layouts. Version 0.4.0 includes 44 new transparent station, hull and component illustrations. Hulls and stations use one shared black starfield derived from the original Stars! art; components use native medium gray. Prompts and asset paths are recorded in docs/artwork-v2-manifest.json; direction is in docs/ART-DIRECTION.txt.

Developers can export production-queue-only plans with `preview-production --database SNAPSHOT.db --game ID --player NUM --output production.json`, and navigation-only plans with `preview-automation --database SNAPSHOT.db --game ID --player NUM --output routes.json`. This command does not apply orders or generate a turn. Never save its scratch simulation as a live game.

The request inventory is `SUGGESTIONS-CHECKLIST.txt`. Upstream authorship and licence remain below.

## Origin and contributions

This project is an open-source derivative of Craig (sirgwain)'s open-source **craig-stars**: https://github.com/sirgwain/craig-stars. Craig's simulation and client form the foundation. Tomek-stars adds single-player automation, fuel logistics, fleet composition and replacement, exploration, expanded maps, race options and technologies. Both projects remain open source under the repository licence. Original Stars! was created by Jeff Johnson and Jeff McBride.

Contributions should include a clear behavior description and relevant tests. Never publish personal saves. Our release series starts at **0.2.0** for the automation edition; increment patch versions for fixes and minor versions for new gameplay capabilities. See SUGGESTIONS-CHECKLIST.txt for implementation status, including unfinished requests.

## 0.4.1: autonomous initiative and turn analysis

Routine routes balance years against actual engine fuel cost and preserve a return budget. Useful combat interception may spend more fuel; emergency returns choose a funded path home even through reported enemy proximity. Engine and minefield checks remain. Free escorts can join patrol interceptions, real merged packs retain multiple ships in a single native battle stack, and packs can recruit more nearby support when the reported enemy grows. Roles resume after the temporary mission.

Ordinary reported scout hulls do not block supply or refuelling routes and do not attract reconnaissance scouts. Unscanned combat strength follows visible ship counts and the hull's Weapon-compatible slot capacities, including General Purpose, scaled to known weapon strength. This is an estimate, not access to hidden components. An unidentified hull receives a finite fallback estimate.

Fuel donors can travel toward slow clients, deliver a useful partial refill, and share spare fuel when ships meet, retaining their own return fuel. A failed rendezvous releases its reservations. Empty transports outside friendly territory withdraw instead of waiting indefinitely for an unavailable escort. Recovery preserves purposeful waits while real rendezvous/rally participants are moving.

Factories/mines before committed manufacturing now receive one-unit neighbor refinement, including paired changes; every explicit queue deadline is protected. Reserved scrap-and-rebuild production follows the newest family version even if another upgrade appears after scrapping, retaining quantities and unspent paid credits. Historical zero-instance designs remain listed.

Local analysis archives are enabled by default with `Game.TurnArchives: true`. Each turn writes compressed JSON before simulation (`orders`) and after the committed result (`result`) under `data/turn-archives/game-ID/`. These contain the complete physical universe, all players' orders/technology/economy and human reports; redundant AI intel caches are omitted. They are analysis records, not a substitute for a full database backup. Archives stay local and are excluded from Git. They accumulate until you remove them.

Independent planet-report decoding overlaps database reading across available Go workers, with a bounded hand-off and prompt raw-buffer release. Planet forecasts also use the Go runtime's available parallelism. Simulation phases with shared RNG, combat and state writes remain ordered; this is not a claim that every part of a turn runs on every core.
