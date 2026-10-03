// Package peoplenest splits approved wages across stores proportionally to
// worked minutes while preserving both per-employee and per-store totals.
package peoplenest

import (
	"math/big"
	"math/bits"
)

// Wage is one employee's approved pay for the period, in cents.
type Wage struct {
	EmployeeID string
	Amount     int64
}

// StoreTarget is one store's settlement target for the period, in cents.
type StoreTarget struct {
	StoreID string
	Amount  int64
}

// WorkLog records minutes one employee worked at one store.
type WorkLog struct {
	EmployeeID string
	StoreID    string
	Minutes    int64
}

// Allocation is the integer amount one employee is paid by one store.
type Allocation struct {
	EmployeeID string
	StoreID    string
	Amount     int64
}

// Solve splits wages across stores proportionally to worked minutes.
//
// Every employee's allocations sum to exactly their wage and every store's
// received total equals its settlement target. For each worked pair the
// allocated amount is the floor or ceil of the theoretical proportional
// amount (unchanged when the theory is already integral). Among all feasible
// matrices Solve minimizes the sum of absolute deviations from the
// theoretical amounts. It returns ok=false when no feasible allocation
// exists.
func Solve(wages []Wage, targets []StoreTarget, logs []WorkLog) ([]Allocation, bool) {
	empIndex := make(map[string]int, len(wages))
	for i, w := range wages {
		empIndex[w.EmployeeID] = i
	}
	storeIndex := make(map[string]int, len(targets))
	for i, t := range targets {
		storeIndex[t.StoreID] = i
	}

	// Aggregate minutes per (employee, store) pair, preserving first-seen
	// order so output is deterministic.
	type pair struct {
		emp, store int
	}
	pairOrder := []pair{}
	pairMinutes := make(map[pair]int64)
	empTotal := make([]int64, len(wages))
	for _, l := range logs {
		ei, eok := empIndex[l.EmployeeID]
		si, sok := storeIndex[l.StoreID]
		if !eok || !sok || l.Minutes <= 0 {
			continue
		}
		p := pair{ei, si}
		if _, seen := pairMinutes[p]; !seen {
			pairOrder = append(pairOrder, p)
		}
		pairMinutes[p] += l.Minutes
		empTotal[ei] += l.Minutes
	}

	// Baseline: every pair gets floor(wage * minutes / totalMinutes).
	// Each pair with a nonzero remainder may contribute one extra cent;
	// choosing it changes the total absolute deviation by
	// (total - 2*rem) / total, which becomes the edge cost.
	numEmps, numStores := len(wages), len(targets)
	source := numEmps + numStores
	sink := source + 1
	mcf := newMinCostFlow(sink + 1)

	empSupply := make([]int64, numEmps)
	for i, w := range wages {
		empSupply[i] = w.Amount
	}
	storeDemand := make([]int64, numStores)
	for i, t := range targets {
		storeDemand[i] = t.Amount
	}

	type edgeRef struct {
		pairIndex int
		from, to  int
	}
	edgeRefs := []edgeRef{}
	floors := make([]int64, len(pairOrder))
	for pi, p := range pairOrder {
		w := wages[p.emp].Amount
		m := pairMinutes[p]
		t := empTotal[p.emp]
		// Exact 128-bit numerator wage*minutes, then divide by total.
		hi, lo := bits.Mul64(uint64(w), uint64(m))
		quo, rem := bits.Div64(hi, lo, uint64(t))
		floors[pi] = int64(quo)
		empSupply[p.emp] -= int64(quo)
		storeDemand[p.store] -= int64(quo)
		if rem == 0 {
			continue
		}
		cost := big.NewRat(t-2*int64(rem), t)
		from, to := p.emp, numEmps+p.store
		mcf.addEdge(from, to, 1, cost)
		edgeRefs = append(edgeRefs, edgeRef{pi, from, to})
	}

	var required int64
	for i, s := range empSupply {
		if s < 0 {
			return nil, false
		}
		mcf.addEdge(source, i, s, new(big.Rat))
		required += s
	}
	for i, d := range storeDemand {
		if d < 0 {
			return nil, false
		}
		mcf.addEdge(numEmps+i, sink, d, new(big.Rat))
	}

	if _, _, ok := mcf.flow(source, sink, required); !ok {
		return nil, false
	}

	// Read back which rounding edges carry flow.
	chosen := make(map[int]bool, len(edgeRefs))
	for _, r := range edgeRefs {
		for _, e := range mcf.graph[r.from] {
			if e.to == r.to && e.cap == 0 {
				chosen[r.pairIndex] = true
			}
		}
	}

	allocs := make([]Allocation, 0, len(pairOrder))
	for pi, p := range pairOrder {
		amount := floors[pi]
		if chosen[pi] {
			amount++
		}
		allocs = append(allocs, Allocation{
			EmployeeID: wages[p.emp].EmployeeID,
			StoreID:    targets[p.store].StoreID,
			Amount:     amount,
		})
	}
	return allocs, true
}
