package cs

import "slices"

// Reconcile persisted reservations before availability filters skip a refitting
// leader. Neither a stale number nor a maintenance request owns ready peers.
func (t *turnGenerator) reconcilePatrolJobs(p *Player) {
	fleets := slices.Clone(t.game.getFleets(p.Num))
	before := map[int]patrolJob{}
	for _, f := range fleets {
		before[f.Num] = readPatrolJob(f)
	}
	for _, f := range fleets {
		j := before[f.Num]
		if j.Leader == 0 {
			continue
		}
		leader := t.game.getFleet(p.Num, j.Leader)
		valid := leader != nil && !leader.Delete
		if valid {
			lj := before[leader.Num]
			valid = lj.Leader == leader.Num && lj.Enemy == j.Enemy && lj.Target == j.Target && (j.LeaderID == 0 || j.LeaderID == leader.ID)
		}
		if !valid || !patrolAvailable(p, f) {
			delete(f.Tags, patrolTag)
			if f.InAutomaticSafety() && j.Grouped {
				savePatrolJob(f, patrolJob{Grouped: true, Status: "Merged patrol retained during retreat/repair; no peer reservation"})
			}
			restorePatrolBattlePlan(p, f)
			if f.GetTag(patrolSupportTag) == "escort" {
				p.Automation.FleetRoles[f.Num] = "escort"
				delete(f.Tags, patrolSupportTag)
			}
		}
	}
	// An unavailable leader's tag may already have been removed above. Recover
	// its real survivors by mission identity, then elect an operational leader.
	for _, f := range fleets {
		j := readPatrolJob(f)
		if j.Leader == 0 {
			continue
		}
		leader := t.game.getFleet(p.Num, j.Leader)
		if leader != nil && patrolAvailable(p, leader) && readPatrolJob(leader).Leader == leader.Num {
			j.LeaderID = leader.ID
			savePatrolJob(f, j)
			continue
		}
		var group []*Fleet
		for _, m := range fleets {
			mj := readPatrolJob(m)
			if patrolAvailable(p, m) && mj.Leader == j.Leader && mj.Enemy == j.Enemy && mj.Target == j.Target {
				group = append(group, m)
			}
		}
		if len(group) == 0 {
			continue
		}
		if original, ok := before[j.Leader]; ok {
			j = original
		}
		slices.SortStableFunc(group, func(a, b *Fleet) int {
			if a.Spec.TotalShips != b.Spec.TotalShips {
				return b.Spec.TotalShips - a.Spec.TotalShips
			}
			return a.Num - b.Num
		})
		t.transferPatrolCommand(p, group[0], &j, group)
	}
}

func (t *turnGenerator) transferPatrolCommand(p *Player, leader *Fleet, job *patrolJob, group []*Fleet) {
	job.Leader, job.LeaderID = leader.Num, leader.ID
	for _, f := range group {
		next := *job
		next.Leader, next.LeaderID = leader.Num, leader.ID
		savePatrolJob(f, next)
	}
}

// Move an actually selected formation to a more urgent task without mutating
// all candidate missions during the read-only composition search.
func (t *turnGenerator) detachPatrolMember(p *Player, f *Fleet) {
	j := readPatrolJob(f)
	if j.Leader == f.Num {
		var survivors []*Fleet
		for _, m := range t.game.getFleets(p.Num) {
			if m != f && patrolAvailable(p, m) && readPatrolJob(m).Leader == f.Num {
				survivors = append(survivors, m)
			}
		}
		if len(survivors) > 0 {
			t.transferPatrolCommand(p, survivors[0], &j, survivors)
		}
	}
	delete(f.Tags, patrolTag)
}
