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
- The map displays the full fuel-budgeted scout petal, including planned stops and the return dock. Navigation remains subject to safety checks each turn.
- New-game settings are remembered locally, including races, opponents and starting location. **Reset to defaults** restores defaults. The slider accepts 0–144 AI; the map must contain enough planets. **Edge/Center** select the human capital location while keeping normal opponent placement rules.
- After a human is publicly declared victor, AI prefer reachable attacks on that victor and prioritize military production while retaining territorial defense. No free resources or hidden intelligence are granted.

The request inventory is `SUGGESTIONS-CHECKLIST.txt`. Upstream authorship and licence remain below.

![screenshot](docs/screenshots/screenshot1.png)

An open-source, web based Stars! clone.

[![Build Status](https://github.com/sirgwain/craig-stars/actions/workflows/build.yml/badge.svg)](https://github.com/sirgwain/craig-stars/actions/workflows/build.yml)

## About

[Stars!](https://wikipedia.org/en/Stars!) is an interstellar, turn based strategy game and a member of the 4X genre (eXplore, eXpand, eXploit, eXterminate). The original Stars! game was released as shareware in 1995 by Jeff Johnson and Jeff McBride with help from Jeffrey Krauss (collectively known as the Jeffs), with a retail version published in 1997. `craig-stars` attempts to remain as close to the original as possible in both UI and gameplay.[^1]

The [Stars! wiki](https://wiki.starsautohost.org/) and [Home World Forum](https://starsautohost.org/sahforum2/) are both great places for learning about Stars! gameplay. The wiki's [strategy guide](https://wiki.starsautohost.org/wikinew/ssg/ssg.htm) is a good starting point for new players to learn the basics, and the active [Stars! discord server](https://discord.gg/Ctdx7h6UZS) is a great place to ask questions and start/join new multiplayer games. (For developers & contributors, there is also a channel for stars clone discussion.)

[^1]: A non-exhaustive list of intentional differences can be found [here](./docs/changes.md).

## Development/Contributing

Contributions of any form are welcome, even non-code ones such as bug reports & feature requests. If you have any issues to report or cool ideas to suggest, open an Issue report on the GitHub repository.

For code contributions, all you need to get started is a good code editor (like Visual Studio Code - available to [download](https://code.visualstudio.com/Download) or [use online](https://vscode.dev)), as well as a GitHub account.

Meanwhile, for those who are more technically inclined and want to test out changes themself before committing, consider running a local instance of `craig-stars` for development. See the [development](docs/development.md) page for more info.

## Architecture

For detailed information about `craig-stars` architecture, check out the [architecture](docs/architecture.md) page.
