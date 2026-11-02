package cs

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
type StatisticsCounters struct {
	ShipsDestroyed             int64 `json:"shipsDestroyed,omitempty"`
	DestroyedTonnage           int64 `json:"destroyedTonnage,omitempty"`
	ShipsLost                  int64 `json:"shipsLost,omitempty"`
	LostTonnage                int64 `json:"lostTonnage,omitempty"`
	ShipsBuilt                 int64 `json:"shipsBuilt,omitempty"`
	BuiltTonnage               int64 `json:"builtTonnage,omitempty"`
	ShipsScrapped              int64 `json:"shipsScrapped,omitempty"`
	ScrappedTonnage            int64 `json:"scrappedTonnage,omitempty"`
	BattlesFought              int64 `json:"battlesFought,omitempty"`
	BattlesWon                 int64 `json:"battlesWon,omitempty"`
	ResourcesProduced          int64 `json:"resourcesProduced,omitempty"`
	ProductionResourcesSpent   int64 `json:"productionResourcesSpent,omitempty"`
	ResearchResourcesSpent     int64 `json:"researchResourcesSpent,omitempty"`
	AllocationRefundsResources int64 `json:"allocationRefundsResources,omitempty"`
	IroniumSpent               int64 `json:"ironiumSpent,omitempty"`
	BoraniumSpent              int64 `json:"boraniumSpent,omitempty"`
	GermaniumSpent             int64 `json:"germaniumSpent,omitempty"`
	IroniumMined               int64 `json:"ironiumMined,omitempty"`
	BoraniumMined              int64 `json:"boraniumMined,omitempty"`
	GermaniumMined             int64 `json:"germaniumMined,omitempty"`
	FactoriesBuilt             int64 `json:"factoriesBuilt,omitempty"`
	MinesBuilt                 int64 `json:"minesBuilt,omitempty"`
	DefensesBuilt              int64 `json:"defensesBuilt,omitempty"`
	MineralsSalvaged           int64 `json:"mineralsSalvaged,omitempty"`
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
func statisticsDifference(a, b StatisticsCounters) StatisticsCounters {
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
		MineralsSalvaged:           a.MineralsSalvaged - b.MineralsSalvaged,
	}
}
func (t *turnGenerator) beginStatistics() {
	t.statisticsBefore = map[int]StatisticsCounters{}
	for _, p := range t.game.Players {
		t.statisticsBefore[p.Num] = *p.accounting(t.game.Year)
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
func (t *turnGenerator) recordBattleStatistics(record *BattleRecord) {
	tokens := map[int]BattleRecordToken{}
	survivors := map[int]int{}
	for _, token := range record.Tokens {
		tokens[token.Num] = token
		survivors[token.PlayerNum] += token.StartingQuantity
	}
	for _, dead := range record.DestroyedTokens {
		p := t.game.getPlayer(dead.PlayerNum)
		if p == nil {
			continue
		}
		c := p.accounting(t.game.Year)
		c.ShipsLost += int64(dead.Quantity)
		mass := tokens[dead.Num].Mass
		if dead.design != nil {
			mass = dead.design.Spec.Mass
		}
		c.LostTonnage += int64(mass) * int64(dead.Quantity)
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
			for _, dead := range record.DestroyedTokens {
				if dead.Num == target.Num && dead.design != nil {
					mass = dead.design.Spec.Mass
					break
				}
			}
			c.DestroyedTonnage += int64(mass) * int64(action.TokensDestroyed)
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
