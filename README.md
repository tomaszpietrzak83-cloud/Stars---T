<!-- Local edition: upstream attribution and licence are preserved. -->

# tomek-stars

A local edition built on **craig-stars**, created by **Craig (sirgwain)**: https://github.com/sirgwain/craig-stars.

## Play on another Windows computer

Download this repository with **Code → Download ZIP**, extract it completely, and double-click **tomek-stars.bat**. The Windows server and compiled interface are included. No Go, Zig, npm, Vite or account registration is needed to play. If a compatible Node.js is unavailable, the launcher downloads a portable LTS runtime from nodejs.org, checks its published SHA-256, and keeps it inside `.runtime` in this game folder.

Open the game at `http://localhost:5173`. The launcher signs into the local game automatically and restarts only processes belonging to this copy. A first start creates a fresh local database. Personal games are stored in `data/data.db`; **this folder is intentionally excluded from Git**. To move an existing game, stop both copies and separately copy the entire `data` folder. Keep a backup before replacing it. Updates replace program files, not `data`.

The source is Go and Svelte/TypeScript. Developers need the Go version from `go.mod`, Node.js 22.17 or newer, and a CGO-capable C compiler. Run `npm ci` in `frontend`; build `wasm/main.go` using `GOOS=js GOARCH=wasm CGO_ENABLED=0` into `frontend/src/lib/wasm/cs.wasm`; run `npm run build` in `frontend`; build the native server with CGO into `windows/stars-local.exe`. The committed edition can be played without rebuilding.

## Version 0.6.6

Patrols assess reported encounter forces with isolated native combat simulations and select mixed reinforcements together with battle tactics. The final departure and bounded pursuit checks reject an unsafe destination even when the original target was a weak transport. Reconnaissance and funded concentration run in parallel; survivor damage, repair downtime, public reinforcement courses, contact memory and real debris pickup are included.

Verification and limits: [docs/AUDIT-0.6.6.txt](docs/AUDIT-0.6.6.txt).

## Version 0.6.5

Free patrols advance from cleared local sectors toward reported frontier formations, balancing reinforcements against threat and existing support. Long unarmed chases yield to active fronts; nearby interception and imminent colony defense remain first. Movement stages outside enemy reach with funded return routes, and shifts farther out as contacts clear.

Verification: [docs/AUDIT-0.6.5.txt](docs/AUDIT-0.6.5.txt).

## Version 0.6.4

Equipment reconnaissance is mandatory for uncertain encounters with eight or more reported ships. Smaller unknown groups remain attackable with sufficient strength. Oversized patrol packs release surplus ships against small unarmed targets; the archived fifteen-destroyer case now sends three and frees twelve. Radiating scoops retain free warp 6 for routine patrols and can use faster funded pursuit.

Verification: [docs/AUDIT-0.6.4.txt](docs/AUDIT-0.6.4.txt).

## Version 0.6.3

Automatic patrols require scout equipment reconnaissance before attacking unknown fleets, including nearby or visibly converging unknown contacts. Existing saved automatic attacks are held too; a priority order cannot bypass reconnaissance. Scouts inspect local patrol targets even when they are not heading toward a colony. Manual control and safety returns remain available.

Verification and limits: [docs/AUDIT-0.6.3.txt](docs/AUDIT-0.6.3.txt).

## Version 0.6.2

Slow observed hostile formations now receive scanner watches, economical patrol containment and nearby debris collectors. Patrols prefer visible small support ships converging on the formation. Radiating scoop patrols gather at free warp 6; fuel, mine safety and committed transport tasks remain protected. Collectors join covered battles with disengagement orders and pick up real debris the next turn.

Verification and limits: [docs/AUDIT-0.6.2.txt](docs/AUDIT-0.6.2.txt).

## Version 0.6.1

Rapid consecutive starbase upgrades now receive the same four-year, 90%-of-old-value allowance as ships. Existing native component-reuse pricing remains available whenever cheaper; the discounts do not stack. The old base remains operational until construction completes.

Ship Designs now shows each ship/base blueprint's creation game year on its card and detail screen. Legacy unknown dates remain marked unknown. Native production and WASM UI forecasts use the same upgrade cost. Verification: [docs/AUDIT-0.6.1.txt](docs/AUDIT-0.6.1.txt).

## Version 0.6.0

Automatic collectors can arrive with a superior patrol, follow its updated battle location and load debris at the beginning of the following turn. Existing debris takes precedence over speculative battle reservations. Scannerless freighters discover debris at physical contact. Annual deep-space decay is reduced from 10%/10 kT to 8%/8 kT per mineral.

Enemy fleets now have a priority crosshair. Patrols favor these destruction orders, pursue beyond their ordinary local radius with sufficient strength and return fuel, and clear them after confirmed destruction. Planet priorities clear when the planet becomes yours. Report links retain the chosen galaxy zoom.

Verification and limits: [docs/AUDIT-0.6.0.txt](docs/AUDIT-0.6.0.txt).

## Version 0.5.0

Rapid consecutive ship upgrades recover **90% of the old ship's resources and minerals** when the next project in the same design family was created within four game years. The successor still costs its full normal price. Skipping a version and ordinary scrapping retain normal recovery. Delivery and return orders finish before rebuilding.

Project dates are recorded from this edition onward; existing designs without known dates use ordinary recovery. Details and verification: [docs/AUDIT-0.5.0.txt](docs/AUDIT-0.5.0.txt).

## Version 0.4.8

The header beside **Submit Turn** shows total annual resources and estimated research in blue, including unused production resources. Automatic ship upgrades and retirements produce one per-turn summary of actual scrapping, replacement construction and completed rebuilds.

Rebuilding preserves the current delivery and funded return. Investigation of the reported transports confirmed normal next-turn rebuilding after arrival; an isolated regression now protects that sequence. Details: [docs/AUDIT-0.4.8.txt](docs/AUDIT-0.4.8.txt).

## Version 0.4.7

New-game options include brief explanations. Edge starts prefer a planet around 5% of the map width or height from the nearest border. Saved setup restores victory thresholds, selected criteria and race choices; Reset to defaults updates the whole form.

Planet and fleet side-panel sections can be collapsed independently and remember their state. Scouts with penetrating scanners plan shared observation stops to cover several planets without entering orbit, while reserving fuel for the return. Enemy-colony refresh and incoming-fleet reconnaissance use actual scanner range and reported motion; equipment discovery still follows the race's normal rules. Verification: [docs/AUDIT-0.4.7.txt](docs/AUDIT-0.4.7.txt).

## Version 0.4.5

Patrols reconsider reported targets each year, predict head-on interception points and use stable sectors around fuel bases. Nearby patrols can physically merge for multi-ship targets, then resume their roles. Planet priorities influence escort and patrol coverage. Gate routes preserve their destination planet and respect real hull-mass, cargo and range limits.

Marking a Mystery Trader requests a one-shot mineral offering from available automatic freighters. The controller reserves actual surplus minerals, assembles and merges the offering, and stages an interception along the reported course. **The native trader rule absorbs the entire offering fleet.** Manual ships and committed planetary production are protected; insufficient capacity or stock does not create a partial offering.

Passenger planning budgets the empty return after delivery and redirects empty waiting transports when another source can provide a full useful load. Rebuilds disperse within owned base sectors without an unnecessary exact-position rally; incoming assignments reserve yard workload and mineral demand before scrapping. Radiating escorts remain separate from passenger fleets.

Request history, audit status and verification are indexed in [docs/AUDIT-REQUESTS.txt](docs/AUDIT-REQUESTS.txt). Private transcripts and saves are not included in the public repository.

## Version 0.4.4

Automatic patrol/escort stacks can split into independent ships and regroup for combat. Weapon-based design strategies include ranged Sniper tactics; the designer previews and edits shared battle plans without leaving the draft. Manual fleet plans override design defaults.

Automatic colony ships without a colonization target can carry passengers, then resume colonization; nearby bombing can trigger staging at a friendly base. Marked enemy colonies receive fresh scout reconnaissance. Tankers prioritize fuel rescue and active combat over optional civilian top-ups. Mature colonies default to 50% research and 50% alchemy, configurable in Automation; ship/base orders restore ordinary taxation. Temporary production investments must preserve every native manufacturing deadline; automatic alchemy requires full factory/mine capacity. Battle summaries emphasize losses, and station markers have larger, outlined silhouettes.

Scanner visibility uses an exact spatial index and bounded parallel work. AI planning runs per owner on available CPU cores. A paired copied-save measurement at year 2476 improved native turn generation from about 184 to 85 seconds; this is one measurement on one machine. Details and limits are in [the performance notes](docs/PERFORMANCE-0.4.4.txt). Personal saves and credentials are never included in this repository.

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

## 0.4.3: manual command priority and faster turns

Manual fleets cannot be recruited as bomber escorts. Disabling automation clears resumable automatic jobs, stale assignments after merging are removed, and legacy manual missions commandeered by the earlier escort controller are restored. Sacrificial fleet reconnaissance uses one real scout; stacked scouts split first. Scouts without unknown planets scan frontier sectors.

Planetary bomb damage is excluded from anti-ship strength estimates. Patrol packs stop waiting indefinitely for distant reinforcements and defer unachievable targets, allowing individual patrols to hunt and regroup. Pack members split again after contact ends. Automation can sort by Rebuilding, and Other Entities Here shows concise order labels for owned fleets.

Civilian pickups can request escort for the delivery leg before loading. Distant threats are bounded by reported interception reach. Fully deliverable cargo is unloaded in the return-fuel forecast; changed/full/foreign destinations retain a loaded return reserve. Useful base mineral deliveries receive a free transporter before routine work. Default development raises annual factory/mine limits to use available production when no ship/base batch takes precedence.

A copied year-2470, 144-player, 30,858-fleet test improved native GenerateTurn wall time from 127.2 to 76.2 seconds on the same machine. Reported-design lookup uses an index. Player planet reports use lossless compressed SQLite blobs, prepared in bounded parallel batches; transactional SQL writes retain their order. Legacy plain JSON remains readable, and APIs/analysis archives remain JSON. **After the first turn saved by 0.4.3, keep using 0.4.3 or later with that database**; keep your separate pre-update backup to return to an older executable. No automatic database vacuum or deletion is performed.

## 0.4.2: design cleanup and civilian protection

Empty superseded designs disappear from the designer while referenced blueprints remain available for production and reports. An unused older version can be deleted without clearing the whole family's maintenance orders; targets and queued production remain protected.

New ships never inherit a destroyed fleet's automation through a reused fleet number. Only actual scout hulls can accept sacrificial contact reconnaissance. Legacy freighters incorrectly carrying a scout role return to transport work, and unarmed cargo ships cannot depart on enemy-fleet interception orders. Patrol pack members merge at intermediate rendezvous before combat. Incoming reported courses toward colonies trigger defensive mobilization beyond the immediate interception radius; outmatched defenders gather rather than charge. Automatic colonizers arriving at an already owned colony unload their settlers and continue with loading or pending retirement.

## 0.4.1: autonomous initiative and turn analysis

Routine routes balance years against actual engine fuel cost and preserve a return budget. Useful combat interception may spend more fuel; emergency returns choose a funded path home even through reported enemy proximity. Engine and minefield checks remain. Free escorts can join patrol interceptions, real merged packs retain multiple ships in a single native battle stack, and packs can recruit more nearby support when the reported enemy grows. Roles resume after the temporary mission.

Ordinary reported scout hulls do not block supply or refuelling routes and do not attract reconnaissance scouts. Unscanned combat strength follows visible ship counts and the hull's Weapon-compatible slot capacities, including General Purpose, scaled to known weapon strength. This is an estimate, not access to hidden components. An unidentified hull receives a finite fallback estimate.

Fuel donors can travel toward slow clients, deliver a useful partial refill, and share spare fuel when ships meet, retaining their own return fuel. A failed rendezvous releases its reservations. Empty transports outside friendly territory withdraw instead of waiting indefinitely for an unavailable escort. Recovery preserves purposeful waits while real rendezvous/rally participants are moving.

Factories/mines before committed manufacturing now receive one-unit neighbor refinement, including paired changes; every explicit queue deadline is protected. Reserved scrap-and-rebuild production follows the newest family version even if another upgrade appears after scrapping, retaining quantities and unspent paid credits. Historical zero-instance designs remain listed.

Local analysis archives are enabled by default with `Game.TurnArchives: true`. Each turn writes compressed JSON before simulation (`orders`) and after the committed result (`result`) under `data/turn-archives/game-ID/`. These contain the complete physical universe, all players' orders/technology/economy and human reports; redundant AI intel caches are omitted. They are analysis records, not a substitute for a full database backup. Archives stay local and are excluded from Git. They accumulate until you remove them.

Independent planet-report decoding overlaps database reading across available Go workers, with a bounded hand-off and prompt raw-buffer release. Planet forecasts also use the Go runtime's available parallelism. Simulation phases with shared RNG, combat and state writes remain ordered; this is not a claim that every part of a turn runs on every core.
