package cs

// Find a sufficient nearby prefix with bounded native forecasts, then narrow
// that confirmed force. This leaves work for other simultaneous relief tasks.
// A failed coarse search falls back to the caller's mixed-composition search;
// no assumption of monotonic combat outcomes can authorize an attack.
func readyPatrolPrefix(p *Player, pool []*Fleet, enemy *Fleet, point Vector, checks *int, limit int) ([]*Fleet, patrolAssessment) {
	e := p.patrolEvaluator
	chosen, failed := 0, 1
	result := patrolAssessment{}
	assess := func(n int) patrolAssessment {
		*checks++
		return e.assess(p, pool[:n], enemy, point, 1)
	}
	for n := min(2, len(pool)); n > 1 && n <= len(pool) && *checks < limit && e.Work < e.Limit; {
		check := assess(n)
		if check.Ready {
			chosen, result = n, check
			break
		}
		failed = n
		if n == len(pool) {
			break
		}
		if n >= max(64, reportedFleetShips(enemy)) {
			// Large unsuccessful formations can consume most of the forecast
			// budget. Probe the complete nearby force before spending the rest
			// on intermediate sizes; every accepted force still needs a native
			// confirmation, and narrowing remains bounded below.
			n = len(pool)
		} else {
			n = min(len(pool), n*2)
		}
	}
	if chosen == 0 {
		return nil, result
	}
	for lo, hi := failed+1, chosen; lo < hi && *checks < limit && e.Work < e.Limit; {
		mid := (lo + hi) / 2
		check := assess(mid)
		if check.Ready {
			chosen, result, hi = mid, check, mid
		} else {
			lo = mid + 1
		}
	}
	return pool[:chosen], result
}
