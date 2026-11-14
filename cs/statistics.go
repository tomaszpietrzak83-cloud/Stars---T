package cs

import "maps"

// Statistics are event-based, persisted in the existing stats JSON column.
// Old saves start detailed tracking on the first newly generated turn.
type PlayerAccounting struct {
	SinceYear int                `json:"sinceYear"`
	Counters  StatisticsCounters `json:"counters"`
	History   []StatisticsYear   `json:"history,omitempty"`
}
type StatisticsYear struct {
	Year     int                `json:"year"`
	Counters StatisticsCounters `json:"counters"`
}
type HullStatistics struct {
	ShipsDestroyed   int64 `json:"shipsDestroyed,omitempty"`
	DestroyedTonnage int64 `json:"destroyedTonnage,omitempty"`
	ShipsLost        int64 `json:"shipsLost,omitempty"`
	LostTonnage      int64 `json:"lostTonnage,omitempty"`
	ShipsBuilt       int64 `json:"shipsBuilt,omitempty"`
	BuiltTonnage     int64 `json:"builtTonnage,omitempty"`
}

type StatisticsCounters struct {
	ShipsDestroyed             int64                     `json:"shipsDestroyed,omitempty"`
	DestroyedTonnage           int64                     `json:"destroyedTonnage,omitempty"`
	ShipsLost                  int64                     `json:"shipsLost,omitempty"`
	LostTonnage                int64                     `json:"lostTonnage,omitempty"`
	ShipsBuilt                 int64                     `json:"shipsBuilt,omitempty"`
	BuiltTonnage               int64                     `json:"builtTonnage,omitempty"`
	ShipsScrapped              int64                     `json:"shipsScrapped,omitempty"`
	ScrappedTonnage            int64                     `json:"scrappedTonnage,omitempty"`
	BattlesFought              int64                     `json:"battlesFought,omitempty"`
	BattlesWon                 int64                     `json:"battlesWon,omitempty"`
	ResourcesProduced          int64                     `json:"resourcesProduced,omitempty"`
	ProductionResourcesSpent   int64                     `json:"productionResourcesSpent,omitempty"`
	ResearchResourcesSpent     int64                     `json:"researchResourcesSpent,omitempty"`
	AllocationRefundsResources int64                     `json:"allocationRefundsResources,omitempty"`
	IroniumSpent               int64                     `json:"ironiumSpent,omitempty"`
	BoraniumSpent              int64                     `json:"boraniumSpent,omitempty"`
	GermaniumSpent             int64                     `json:"germaniumSpent,omitempty"`
	IroniumMined               int64                     `json:"ironiumMined,omitempty"`
	BoraniumMined              int64                     `json:"boraniumMined,omitempty"`
	GermaniumMined             int64                     `json:"germaniumMined,omitempty"`
	FactoriesBuilt             int64                     `json:"factoriesBuilt,omitempty"`
	MinesBuilt                 int64                     `json:"minesBuilt,omitempty"`
	DefensesBuilt              int64                     `json:"defensesBuilt,omitempty"`
	MineralsSalvaged           int64                     `json:"mineralsSalvaged,omitempty"`
	Hulls                      map[string]HullStatistics `json:"hulls,omitempty"`
	IroniumSalvaged            int64                     `json:"ironiumSalvaged,omitempty"`
	BoraniumSalvaged           int64                     `json:"boraniumSalvaged,omitempty"`
	GermaniumSalvaged          int64                     `json:"germaniumSalvaged,omitempty"`
}

func (p *Player) accounting(year int) *StatisticsCounters {
	if p.Stats == nil {
		p.Stats = &PlayerStats{}
	}
	if p.Stats.Accounting == nil {
		p.Stats.Accounting = &PlayerAccounting{SinceYear: max(1, year)}
	}
	return &p.Stats.Accounting.Counters
}
func (c *StatisticsCounters) recordHull(design *ShipDesign, delta HullStatistics) {
	if design == nil || design.Hull == "" {
		return
	}
	if c.Hulls == nil {
		c.Hulls = make(map[string]HullStatistics)
	}
	old := c.Hulls[design.Hull]
	old.ShipsDestroyed += delta.ShipsDestroyed
	old.DestroyedTonnage += delta.DestroyedTonnage
	old.ShipsLost += delta.ShipsLost
	old.LostTonnage += delta.LostTonnage
	old.ShipsBuilt += delta.ShipsBuilt
	old.BuiltTonnage += delta.BuiltTonnage
	c.Hulls[design.Hull] = old
}

func hullDifference(a, b HullStatistics) HullStatistics {
	return HullStatistics{
		ShipsDestroyed:   a.ShipsDestroyed - b.ShipsDestroyed,
		DestroyedTonnage: a.DestroyedTonnage - b.DestroyedTonnage,
		ShipsLost:        a.ShipsLost - b.ShipsLost,
		LostTonnage:      a.LostTonnage - b.LostTonnage,
		ShipsBuilt:       a.ShipsBuilt - b.ShipsBuilt,
		BuiltTonnage:     a.BuiltTonnage - b.BuiltTonnage,
	}
}

func statisticsDifference(a, b StatisticsCounters) StatisticsCounters {
	var hulls map[string]HullStatistics
	for hull, after := range a.Hulls {
		if delta := hullDifference(after, b.Hulls[hull]); delta != (HullStatistics{}) {
			if hulls == nil {
				hulls = make(map[string]HullStatistics)
			}
			hulls[hull] = delta
		}
	}
	return StatisticsCounters{
		ShipsDestroyed:             a.ShipsDestroyed - b.ShipsDestroyed,
		DestroyedTonnage:           a.DestroyedTonnage - b.DestroyedTonnage,
		ShipsLost:                  a.ShipsLost - b.ShipsLost,
		LostTonnage:                a.LostTonnage - b.LostTonnage,
		ShipsBuilt:                 a.ShipsBuilt - b.ShipsBuilt,
		BuiltTonnage:               a.BuiltTonnage - b.BuiltTonnage,
		ShipsScrapped:              a.ShipsScrapped - b.ShipsScrapped,
		ScrappedTonnage:            a.ScrappedTonnage - b.ScrappedTonnage,
		BattlesFought:              a.BattlesFought - b.BattlesFought,
		BattlesWon:                 a.BattlesWon - b.BattlesWon,
		ResourcesProduced:          a.ResourcesProduced - b.ResourcesProduced,
		ProductionResourcesSpent:   a.ProductionResourcesSpent - b.ProductionResourcesSpent,
		ResearchResourcesSpent:     a.ResearchResourcesSpent - b.ResearchResourcesSpent,
		AllocationRefundsResources: a.AllocationRefundsResources - b.AllocationRefundsResources,
		IroniumSpent:               a.IroniumSpent - b.IroniumSpent,
		BoraniumSpent:              a.BoraniumSpent - b.BoraniumSpent,
		GermaniumSpent:             a.GermaniumSpent - b.GermaniumSpent,
		IroniumMined:               a.IroniumMined - b.IroniumMined,
		BoraniumMined:              a.BoraniumMined - b.BoraniumMined,
		GermaniumMined:             a.GermaniumMined - b.GermaniumMined,
		FactoriesBuilt:             a.FactoriesBuilt - b.FactoriesBuilt,
		MinesBuilt:                 a.MinesBuilt - b.MinesBuilt,
		DefensesBuilt:              a.DefensesBuilt - b.DefensesBuilt,
		Hulls:                      hulls,
		IroniumSalvaged:            a.IroniumSalvaged - b.IroniumSalvaged,
		BoraniumSalvaged:           a.BoraniumSalvaged - b.BoraniumSalvaged,
		GermaniumSalvaged:          a.GermaniumSalvaged - b.GermaniumSalvaged,
		MineralsSalvaged:           a.MineralsSalvaged - b.MineralsSalvaged,
	}
}
func (t *turnGenerator) beginStatistics() {
	t.statisticsBefore = map[int]StatisticsCounters{}
	for _, p := range t.game.Players {
		before := *p.accounting(t.game.Year)
		before.Hulls = maps.Clone(before.Hulls)
		t.statisticsBefore[p.Num] = before
	}
}
func (t *turnGenerator) finishStatistics() {
	for _, p := range t.game.Players {
		c := p.accounting(t.game.Year)
		h := p.Stats.Accounting.History
		row := StatisticsYear{Year: t.game.Year, Counters: statisticsDifference(*c, t.statisticsBefore[p.Num])}
		if len(h) > 0 && h[len(h)-1].Year == t.game.Year {
			h[len(h)-1] = row
		} else {
			h = append(h, row)
		}
		if len(h) > 200 {
			h = append([]StatisticsYear(nil), h[len(h)-200:]...)
		}
		p.Stats.Accounting.History = h
	}
}
func (t *turnGenerator) recordMined(p *Player, minerals Mineral) {
	c := p.accounting(t.game.Year)
	c.IroniumMined += int64(max(0, minerals.Ironium))
	c.BoraniumMined += int64(max(0, minerals.Boranium))
	c.GermaniumMined += int64(max(0, minerals.Germanium))
}
func recordProductionSpending(c *StatisticsCounters, delta Cost) {
	c.ProductionResourcesSpent += int64(max(0, delta.Resources))
	c.AllocationRefundsResources += int64(max(0, -delta.Resources))
	c.IroniumSpent += int64(max(0, delta.Ironium))
	c.BoraniumSpent += int64(max(0, delta.Boranium))
	c.GermaniumSpent += int64(max(0, delta.Germanium))
}
func (p *Player) recordRecoveredBattleMinerals(year int, minerals Mineral) {
	if minerals == (Mineral{}) {
		return
	}
	c := p.accounting(year)
	c.MineralsSalvaged += int64(minerals.Total())
	c.IroniumSalvaged += int64(minerals.Ironium)
	c.BoraniumSalvaged += int64(minerals.Boranium)
	c.GermaniumSalvaged += int64(minerals.Germanium)
}

func (t *turnGenerator) recordBattleStatistics(record *BattleRecord) {
	tokens := map[int]BattleRecordToken{}
	survivors := map[int]int{}
	for _, token := range record.Tokens {
		tokens[token.Num] = token
		survivors[token.PlayerNum] += token.StartingQuantity
	}
	destroyedDesigns := make(map[int]*ShipDesign, len(record.DestroyedTokens))
	for _, dead := range record.DestroyedTokens {
		p := t.game.getPlayer(dead.PlayerNum)
		if p == nil {
			continue
		}
		c := p.accounting(t.game.Year)
		c.ShipsLost += int64(dead.Quantity)
		design := dead.design
		if design == nil {
			design = p.GetDesign(tokens[dead.Num].DesignNum)
		}
		mass := tokens[dead.Num].Mass
		if design != nil {
			mass = design.Spec.Mass
			destroyedDesigns[dead.Num] = design
		}
		tonnage := int64(mass) * int64(dead.Quantity)
		c.LostTonnage += tonnage
		c.recordHull(design, HullStatistics{ShipsLost: int64(dead.Quantity), LostTonnage: tonnage})
		survivors[dead.PlayerNum] -= dead.Quantity
	}
	for _, round := range record.ActionsPerRound {
		for _, action := range round {
			attacker, ok := tokens[action.TokenNum]
			if !ok || action.TokensDestroyed <= 0 {
				continue
			}
			target, ok := tokens[action.TargetNum]
			if !ok || target.PlayerNum == attacker.PlayerNum {
				continue
			}
			p := t.game.getPlayer(attacker.PlayerNum)
			if p == nil || p.IsFriend(target.PlayerNum) {
				continue
			}
			c := p.accounting(t.game.Year)
			c.ShipsDestroyed += int64(action.TokensDestroyed)
			mass := target.Mass
			design := destroyedDesigns[target.Num]
			if design != nil {
				mass = design.Spec.Mass
			}
			tonnage := int64(mass) * int64(action.TokensDestroyed)
			c.DestroyedTonnage += tonnage
			c.recordHull(design, HullStatistics{ShipsDestroyed: int64(action.TokensDestroyed), DestroyedTonnage: tonnage})
		}
	}
	for owner, alive := range survivors {
		p := t.game.getPlayer(owner)
		if p == nil {
			continue
		}
		c := p.accounting(t.game.Year)
		c.BattlesFought++
		won := alive > 0
		for enemy, count := range survivors {
			if enemy != owner && !p.IsFriend(enemy) && count > 0 {
				won = false
			}
		}
		if won {
			c.BattlesWon++
		}
	}
}
