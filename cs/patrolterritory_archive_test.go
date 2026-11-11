package cs

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArchivedTerritorialPatrolTurns(t *testing.T) {
	root := os.Getenv("STARS_PATROL_ARCHIVES")
	if root == "" {
		t.Skip("optional campaign replay")
	}
	first, last := 2446, 2460
	if os.Getenv("STARS_SWEEP_REPLAY") == "1" {
		first, last = 2476, 2479
	}
	if year := os.Getenv("STARS_PATROL_YEAR"); year != "" {
		var err error
		first, err = strconv.Atoi(year)
		require.NoError(t, err)
		last = first
	}
	prior := slog.Default()
	defer slog.SetDefault(prior)
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if os.Getenv("STARS_PATROL_DEBUG") != "" {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}
	for year := last; year >= first; year-- {
		t.Run(fmt.Sprint(year+1), func(t *testing.T) {
			files, err := filepath.Glob(filepath.Join(root, fmt.Sprintf("year-%d-orders-*.json.gz", year)))
			require.NoError(t, err)
			require.Len(t, files, 1)
			f, err := os.Open(files[0])
			require.NoError(t, err)
			defer f.Close()
			z, err := gzip.NewReader(f)
			require.NoError(t, err)
			defer z.Close()
			var archive struct{ Game *FullGame }
			require.NoError(t, json.NewDecoder(z).Decode(&archive))
			g := archive.Game
			require.EqualValues(t, 21, g.ID)
			require.Equal(t, year, g.Year)
			g.TechStore = &StaticTechStore
			g.Rules.SetTechStore(&StaticTechStore)
			g.Rules.ResetSeed(int64(year))
			for _, other := range g.Players {
				if len(other.ScoreIntels) == 0 {
					other.ScoreIntels = make([]ScoreIntel, len(g.Players))
				}
				if len(other.PlayerIntels) == 0 {
					other.PlayerIntels = other.defaultPlayerIntels(g.Players)
				}
				if len(other.PlanetIntels) == 0 {
					require.NoError(t, other.initDefaultPlanetIntels(g.Planets))
				}
			}
			tg := newTurnGenerator(g)
			require.NoError(t, g.computeSpecs())
			p := g.getPlayer(1)
			for _, m := range makePatrolMissionQueue(p, year) {
				if m.Kind != "attack" {
					continue
				}
				enemy := patrolEnemy(p, patrolJob{Enemy: m.Enemy, Target: m.Target})
				if m.Tier > 0 && !priorityFleet(p, enemy) && !(m.Tier == 1 && m.Deadline <= 2 && patrolReportedBomber(p, enemy)) {
					continue
				}
				t.Logf("THREAT %s %s at %v tier=%d deadline=%d marked=%t recon=%t encounter=%d", m.Key, enemy.Name, enemy.Position, m.Tier, m.Deadline, priorityFleet(p, enemy), patrolNeedsRecon(p, enemy), len(patrolEncounterReports(p, enemy, patrolInterceptPosition(enemy), 1)))
			}
			t.Logf("MARKED %d", len(p.Automation.PriorityFleets))
			require.NoError(t, tg.generateTurn())
			if os.Getenv("STARS_SWEEP_REPLAY") == "1" {
				logArchivedMineSwap(t, g, p)
			}
			e := p.patrolEvaluator
			if e != nil {
				t.Logf("WORK %d/%d", e.Work, e.Limit)
			}
			jobs := map[string]int{}
			for _, f := range g.getFleets(p.Num) {
				j := readPatrolJob(f)
				if j.Leader != 0 {
					jobs[priorityFleetKey(j.Enemy, j.Target)]++
				}
			}
			bombs := map[string]int{}
			for _, m := range p.Messages {
				if m.Type == PlayerMessagePlanetBombed {
					bombs[m.TargetName]++
				}
			}
			for _, b := range p.BattleRecords {
				if b.PlanetNum != 0 {
					t.Logf("ORBIT %s %v %v", g.getPlanet(b.PlanetNum).Name, b.Stats.NumShipsByPlayer, b.Stats.ShipsDestroyedByPlayer)
				}
			}
			t.Logf("RESULT bombs=%v jobs=%v battles=%d", bombs, jobs, len(p.BattleRecords))
			if os.Getenv("STARS_MINELAYER_CONTINUE") == "1" {
				logMinelayerPursuit(t, g, p)
				for range 4 {
					removeDeleted(g.Universe)
					g.buildMaps(g.Players)
					require.NoError(t, tg.generateTurn())
					logMinelayerPursuit(t, g, p)
				}
			}
			if os.Getenv("STARS_PATROL_CONTINUE") == "1" {
				for i := 0; i < 3; i++ {
					removeDeleted(g.Universe)
					g.buildMaps(g.Players)
					require.NoError(t, tg.generateTurn())
					for _, b := range p.BattleRecords {
						if b.PlanetNum != 0 {
							t.Logf("CONTINUE %d ORBIT %s %v %v", g.Year, g.getPlanet(b.PlanetNum).Name, b.Stats.NumShipsByPlayer, b.Stats.ShipsDestroyedByPlayer)
						}
					}
					pending := 0
					for _, f := range g.getFleets(1) {
						j := readPatrolJob(f)
						if j.Enemy == 6 && j.Target == 60 {
							pending += f.Spec.TotalShips
						}
					}
					t.Logf("CONTINUE %d Ananke assignment=%d", g.Year, pending)
				}
			}
		})
	}
}

func logArchivedMineSwap(t *testing.T, g *FullGame, p *Player) {
	targets := map[string]int{}
	remaining := map[string]int{}
	rebuilding, deferred, reserve, mines := 0, 0, 0, 0
	for _, field := range g.Minefields {
		if reportedHostileMinefield(p, field) && p.GetMinefieldIntel(field.PlayerNum, field.Num) != nil {
			remaining[minefieldAssignmentKey(field)] = field.NumMines
		}
	}
	for _, f := range g.getFleets(p.Num) {
		if j := readPatrolJob(f); strings.HasPrefix(j.Status, "Intercepting reported minelayer") {
			target := g.getFleet(j.Enemy, j.Target)
			t.Logf("MINELAYER patrol=%d enemy=%d:%d arrived=%t destroyed=%t", f.Num, j.Enemy, j.Target, f.Position == j.Rally, target == nil || target.Delete)
		}
		if !automaticMinesweeper(p, f) || f.Delete {
			continue
		}
		switch {
		case f.InDesignMaintenance():
			rebuilding++
		case f.GetTag("maintenance-pending") != "":
			deferred++
		case f.GetTag(minesweeperTargetTag) != "":
			targets[f.GetTag(minesweeperTargetTag)]++
		default:
			reserve++
		}
		n := 0
		_, _ = fmt.Sscanf(f.GetTag("last-mine-sweep"), "Year %d result: %d", new(int), &n)
		mines += n
	}
	t.Logf("SWEEP year=%d mines=%d targets=%v rebuild=%d pending=%d reserve=%d remaining=%v", g.Year, mines, targets, rebuilding, deferred, reserve, remaining)
}

func logMinelayerPursuit(t *testing.T, g *FullGame, p *Player) {
	watched := map[string]int{}
	patrols := map[string]int{}
	for _, f := range g.getFleets(p.Num) {
		if f.Delete {
			continue
		}
		if j := readScoutRecon(f); j != nil && j.Minelayer {
			key := priorityFleetKey(j.Enemy, j.Target)
			watched[key]++
			t.Logf("WATCH %d scout=%d position=%v target=%s", g.Year, f.Num, f.Position, key)
		}
		if j := readPatrolJob(f); j.Minelayer && j.Leader != 0 {
			key := priorityFleetKey(j.Enemy, j.Target)
			patrols[key]++
			if j.Leader == f.Num {
				t.Logf("CHASE %d patrol=%d position=%v target=%s status=%s", g.Year, f.Num, f.Position, key, j.Status)
			}
		}
	}
	for key, count := range watched {
		require.Equal(t, 1, count, "more than one scout assigned to %s", key)
	}
	for _, b := range p.BattleRecords {
		for _, tok := range b.Tokens {
			if d := p.GetShipDesignIntel(tok.PlayerNum, tok.DesignNum); d != nil && (d.Hull == MiniMineLayer.Name || d.Hull == SuperMineLayer.Name) {
				t.Logf("MINE BATTLE %d enemy=%d design=%d position=%v enemy losses=%d", g.Year, tok.PlayerNum, tok.DesignNum, b.Position, b.Stats.ShipsDestroyedByPlayer[tok.PlayerNum])
			}
		}
	}
	t.Logf("PURSUIT %d watched=%v patrols=%v", g.Year, watched, patrols)
}
