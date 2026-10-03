package peoplenest

import (
	"math/big"
	"math/rand"
	"testing"
)

func allocAmount(allocs []Allocation, emp, store string) (int64, bool) {
	for _, a := range allocs {
		if a.EmployeeID == emp && a.StoreID == store {
			return a.Amount, true
		}
	}
	return 0, false
}

// The example from the specification: two employees paid 1 cent each.
// 甲 works 3 min at A and 2 at B; 乙 works 7 at A and 3 at C.
// Targets A=1, B=1, C=0 force 甲->B and 乙->A.
func TestSolveSpecifiedExample(t *testing.T) {
	wages := []Wage{{EmployeeID: "甲", Amount: 1}, {EmployeeID: "乙", Amount: 1}}
	targets := []StoreTarget{{StoreID: "A", Amount: 1}, {StoreID: "B", Amount: 1}, {StoreID: "C", Amount: 0}}
	logs := []WorkLog{
		{EmployeeID: "甲", StoreID: "A", Minutes: 3},
		{EmployeeID: "甲", StoreID: "B", Minutes: 2},
		{EmployeeID: "乙", StoreID: "A", Minutes: 7},
		{EmployeeID: "乙", StoreID: "C", Minutes: 3},
	}
	allocs, ok := Solve(wages, targets, logs)
	if !ok {
		t.Fatal("expected feasible")
	}
	if v, _ := allocAmount(allocs, "甲", "A"); v != 0 {
		t.Errorf("甲->A = %d, want 0", v)
	}
	if v, _ := allocAmount(allocs, "甲", "B"); v != 1 {
		t.Errorf("甲->B = %d, want 1", v)
	}
	if v, _ := allocAmount(allocs, "乙", "A"); v != 1 {
		t.Errorf("乙->A = %d, want 1", v)
	}
	if v, _ := allocAmount(allocs, "乙", "C"); v != 0 {
		t.Errorf("乙->C = %d, want 0", v)
	}
}

// Totals match and every column target lies within the floor/ceil of its
// column's theoretical total, yet the work-log connectivity makes any
// integer allocation impossible: employee 2 must round up at both A and B
// but has only one cent of rounding budget, and employee 1 can only round
// at C or D, which need nothing.
func TestSolveInfeasibleDespiteColumnRanges(t *testing.T) {
	wages := []Wage{
		{EmployeeID: "e0", Amount: 5},
		{EmployeeID: "e1", Amount: 1},
		{EmployeeID: "e2", Amount: 5},
	}
	targets := []StoreTarget{
		{StoreID: "A", Amount: 3},
		{StoreID: "B", Amount: 2},
		{StoreID: "C", Amount: 6},
		{StoreID: "D", Amount: 0},
	}
	logs := []WorkLog{
		{EmployeeID: "e0", StoreID: "A", Minutes: 1},
		{EmployeeID: "e0", StoreID: "C", Minutes: 4},
		{EmployeeID: "e1", StoreID: "C", Minutes: 2},
		{EmployeeID: "e1", StoreID: "D", Minutes: 2},
		{EmployeeID: "e2", StoreID: "A", Minutes: 3},
		{EmployeeID: "e2", StoreID: "B", Minutes: 2},
		{EmployeeID: "e2", StoreID: "C", Minutes: 4},
	}
	if _, ok := Solve(wages, targets, logs); ok {
		t.Fatal("expected infeasible")
	}
}

// bruteForce enumerates every {floor,ceil} matrix and returns feasibility
// plus the minimum total absolute deviation from the theoretical amounts.
func bruteForce(wages []Wage, targets []StoreTarget, logs []WorkLog) (bool, *big.Rat) {
	type pair struct {
		emp, store int
		minutes    int64
	}
	empIndex := map[string]int{}
	for i, w := range wages {
		empIndex[w.EmployeeID] = i
	}
	storeIndex := map[string]int{}
	for i, s := range targets {
		storeIndex[s.StoreID] = i
	}
	var pairs []pair
	empTotal := make([]int64, len(wages))
	for _, l := range logs {
		pairs = append(pairs, pair{empIndex[l.EmployeeID], storeIndex[l.StoreID], l.Minutes})
		empTotal[empIndex[l.EmployeeID]] += l.Minutes
	}
	n := len(pairs)
	floors := make([]int64, n)
	ceils := make([]int64, n)
	theory := make([]*big.Rat, n)
	for i, p := range pairs {
		w := wages[p.emp].Amount
		num := w * p.minutes
		floors[i] = num / empTotal[p.emp]
		ceils[i] = floors[i]
		if num%empTotal[p.emp] != 0 {
			ceils[i]++
		}
		theory[i] = big.NewRat(w*p.minutes, empTotal[p.emp])
	}
	feasible := false
	best := new(big.Rat)
	values := make([]int64, n)
	var rec func(i int, remE, remS []int64)
	rec = func(i int, remE, remS []int64) {
		if i == n {
			for _, r := range remE {
				if r != 0 {
					return
				}
			}
			for _, r := range remS {
				if r != 0 {
					return
				}
			}
			dev := new(big.Rat)
			for j := range values {
				d := new(big.Rat).SetInt64(values[j])
				d.Sub(d, theory[j])
				dev.Add(dev, d.Abs(d))
			}
			if !feasible || dev.Cmp(best) < 0 {
				best.Set(dev)
			}
			feasible = true
			return
		}
		p := pairs[i]
		for _, v := range [2]int64{floors[i], ceils[i]} {
			if remE[p.emp] >= v && remS[p.store] >= v {
				remE[p.emp] -= v
				remS[p.store] -= v
				values[i] = v
				rec(i+1, remE, remS)
				remE[p.emp] += v
				remS[p.store] += v
			}
		}
	}
	remE := make([]int64, len(wages))
	for i, w := range wages {
		remE[i] = w.Amount
	}
	remS := make([]int64, len(targets))
	for i, s := range targets {
		remS[i] = s.Amount
	}
	rec(0, remE, remS)
	return feasible, best
}

// totalDeviation computes the exact total absolute deviation of an
// allocation matrix from the theoretical amounts.
func totalDeviation(t *testing.T, wages []Wage, targets []StoreTarget, logs []WorkLog, allocs []Allocation) *big.Rat {
	t.Helper()
	empIndex := map[string]int{}
	for i, w := range wages {
		empIndex[w.EmployeeID] = i
	}
	empTotal := make([]int64, len(wages))
	for _, l := range logs {
		empTotal[empIndex[l.EmployeeID]] += l.Minutes
	}
	dev := new(big.Rat)
	seen := map[[2]string]bool{}
	for _, l := range logs {
		key := [2]string{l.EmployeeID, l.StoreID}
		if seen[key] {
			continue
		}
		seen[key] = true
		amount, found := allocAmount(allocs, l.EmployeeID, l.StoreID)
		if !found {
			t.Fatalf("missing allocation for %v", key)
		}
		ei := empIndex[l.EmployeeID]
		theory := big.NewRat(wages[ei].Amount*l.Minutes, empTotal[ei])
		d := new(big.Rat).SetInt64(amount)
		d.Sub(d, theory)
		dev.Add(dev, d.Abs(d))
	}
	return dev
}

// checkInvariants verifies row sums, column sums, and the floor/ceil rule.
func checkInvariants(t *testing.T, wages []Wage, targets []StoreTarget, logs []WorkLog, allocs []Allocation) {
	t.Helper()
	empSum := map[string]int64{}
	storeSum := map[string]int64{}
	empIndex := map[string]int{}
	for i, w := range wages {
		empIndex[w.EmployeeID] = i
	}
	empTotal := make([]int64, len(wages))
	for _, l := range logs {
		empTotal[empIndex[l.EmployeeID]] += l.Minutes
	}
	for _, a := range allocs {
		empSum[a.EmployeeID] += a.Amount
		storeSum[a.StoreID] += a.Amount
	}
	for _, w := range wages {
		if empSum[w.EmployeeID] != w.Amount {
			t.Fatalf("employee %s sum = %d, want %d", w.EmployeeID, empSum[w.EmployeeID], w.Amount)
		}
	}
	for _, s := range targets {
		if storeSum[s.StoreID] != s.Amount {
			t.Fatalf("store %s sum = %d, want %d", s.StoreID, storeSum[s.StoreID], s.Amount)
		}
	}
	seen := map[[2]string]bool{}
	for _, l := range logs {
		key := [2]string{l.EmployeeID, l.StoreID}
		if seen[key] {
			continue
		}
		seen[key] = true
		amount, _ := allocAmount(allocs, l.EmployeeID, l.StoreID)
		ei := empIndex[l.EmployeeID]
		num := wages[ei].Amount * l.Minutes
		floor := num / empTotal[ei]
		ceil := floor
		if num%empTotal[ei] != 0 {
			ceil++
		}
		if amount != floor && amount != ceil {
			t.Fatalf("pair %v amount %d not in {%d, %d}", key, amount, floor, ceil)
		}
	}
}

// TestSolveExhaustive cross-checks Solve against brute-force enumeration on
// many small sparse instances, comparing both feasibility and optimality.
func TestSolveExhaustive(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	checked := 0
	infeasibleSeen := 0
	for checked < 1200 {
		numEmps := 1 + rng.Intn(3)
		numStores := 1 + rng.Intn(3)
		wages := make([]Wage, numEmps)
		targets := make([]StoreTarget, numStores)
		var logs []WorkLog
		empMinutes := make([]int64, numEmps)
		for e := 0; e < numEmps; e++ {
			wages[e] = Wage{EmployeeID: string(rune('a' + e)), Amount: int64(rng.Intn(5))}
			for s := 0; s < numStores; s++ {
				if rng.Intn(2) == 0 {
					m := int64(1 + rng.Intn(4))
					logs = append(logs, WorkLog{
						EmployeeID: wages[e].EmployeeID,
						StoreID:    string(rune('A' + s)),
						Minutes:    m,
					})
					empMinutes[e] += m
				}
			}
			if empMinutes[e] == 0 {
				logs = append(logs, WorkLog{
					EmployeeID: wages[e].EmployeeID,
					StoreID:    string(rune('A' + rng.Intn(numStores))),
					Minutes:    int64(1 + rng.Intn(4)),
				})
			}
		}
		var total int64
		for _, w := range wages {
			total += w.Amount
		}
		// Random non-negative store targets summing to the total wage.
		remaining := total
		for s := 0; s < numStores; s++ {
			amount := remaining
			if s < numStores-1 && remaining > 0 {
				amount = int64(rng.Intn(int(remaining) + 1))
			}
			targets[s] = StoreTarget{StoreID: string(rune('A' + s)), Amount: amount}
			remaining -= amount
		}
		if len(logs) > 12 {
			continue
		}
		checked++

		wantFeasible, wantDev := bruteForce(wages, targets, logs)
		allocs, ok := Solve(wages, targets, logs)
		if ok != wantFeasible {
			t.Fatalf("instance %d: feasibility = %v, want %v\nwages=%v\ntargets=%v\nlogs=%v",
				checked, ok, wantFeasible, wages, targets, logs)
		}
		if !ok {
			infeasibleSeen++
			continue
		}
		checkInvariants(t, wages, targets, logs, allocs)
		gotDev := totalDeviation(t, wages, targets, logs, allocs)
		if gotDev.Cmp(wantDev) != 0 {
			t.Fatalf("instance %d: deviation = %v, want %v\nwages=%v\ntargets=%v\nlogs=%v\nallocs=%v",
				checked, gotDev, wantDev, wages, targets, logs, allocs)
		}
	}
	if infeasibleSeen == 0 {
		t.Fatal("expected at least one infeasible random instance")
	}
	t.Logf("checked %d instances, %d infeasible", checked, infeasibleSeen)
}
