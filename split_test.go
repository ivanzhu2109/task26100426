package peoplenest

import (
	"math/rand"
	"testing"
)

// theoretical returns numerator and denominator of the theoretical amount
// for one work record: wage * minutes / totalMinutes.
func theoretical(emp Employee, rec WorkRecord) (int64, int64) {
	var total int64
	for _, w := range emp.Works {
		total += w.Minutes
	}
	return emp.Wage * rec.Minutes, total
}

// scaledDeviation sums |amount*denom - numerator| over all worked pairs.
func scaledDeviation(employees []Employee, matrix map[string]map[string]int64) int64 {
	var dev int64
	for _, emp := range employees {
		for _, w := range emp.Works {
			num, den := theoretical(emp, w)
			got := matrix[emp.ID][w.StoreID]
			d := got*den - num
			if d < 0 {
				d = -d
			}
			dev += d
		}
	}
	return dev
}

// checkMatrix verifies all hard constraints and returns the scaled deviation.
func checkMatrix(t *testing.T, employees []Employee, stores []Store, matrix map[string]map[string]int64) int64 {
	t.Helper()
	for _, emp := range employees {
		var sum int64
		for _, w := range emp.Works {
			got, ok := matrix[emp.ID][w.StoreID]
			if !ok {
				t.Fatalf("missing pair %s/%s in matrix", emp.ID, w.StoreID)
			}
			num, den := theoretical(emp, w)
			floor := num / den
			if got != floor && got != floor+1 {
				t.Fatalf("pair %s/%s: got %d, want floor/ceil of %d/%d", emp.ID, w.StoreID, got, num, den)
			}
			sum += got
		}
		if sum != emp.Wage {
			t.Fatalf("employee %s: split sums to %d, want wage %d", emp.ID, sum, emp.Wage)
		}
		if len(matrix[emp.ID]) != len(emp.Works) {
			t.Fatalf("employee %s: matrix row has extra pairs", emp.ID)
		}
	}
	for _, s := range stores {
		var sum int64
		for _, emp := range employees {
			sum += matrix[emp.ID][s.ID]
		}
		if sum != s.Target {
			t.Fatalf("store %s: received %d, want target %d", s.ID, sum, s.Target)
		}
	}
	return scaledDeviation(employees, matrix)
}

// bruteForce enumerates every floor/ceil choice per worked pair and returns
// the minimum scaled deviation among feasible matrices.
func bruteForce(employees []Employee, stores []Store) (int64, bool) {
	type pair struct{ emp, rec int }
	var pairs []pair
	for e, emp := range employees {
		for r := range emp.Works {
			pairs = append(pairs, pair{e, r})
		}
	}
	amounts := make([][]int64, len(employees))
	for e, emp := range employees {
		amounts[e] = make([]int64, len(emp.Works))
	}
	storeIdx := make(map[string]int)
	for i, s := range stores {
		storeIdx[s.ID] = i
	}
	best := int64(-1)
	var walk func(k int)
	walk = func(k int) {
		if k == len(pairs) {
			for e, emp := range employees {
				var sum int64
				for _, a := range amounts[e] {
					sum += a
				}
				if sum != emp.Wage {
					return
				}
			}
			colSums := make([]int64, len(stores))
			for e, emp := range employees {
				for r, w := range emp.Works {
					colSums[storeIdx[w.StoreID]] += amounts[e][r]
				}
			}
			for i, s := range stores {
				if colSums[i] != s.Target {
					return
				}
			}
			var dev int64
			for e, emp := range employees {
				for r, w := range emp.Works {
					num, den := theoretical(emp, w)
					d := amounts[e][r]*den - num
					if d < 0 {
						d = -d
					}
					dev += d
				}
			}
			if best < 0 || dev < best {
				best = dev
			}
			return
		}
		p := pairs[k]
		num, den := theoretical(employees[p.emp], employees[p.emp].Works[p.rec])
		amounts[p.emp][p.rec] = num / den
		walk(k + 1)
		amounts[p.emp][p.rec] = num/den + 1
		walk(k + 1)
	}
	walk(0)
	return best, best >= 0
}

func TestSolveSpecifiedCase(t *testing.T) {
	employees := []Employee{
		{ID: "甲", Wage: 1, Works: []WorkRecord{{StoreID: "A", Minutes: 3}, {StoreID: "B", Minutes: 2}}},
		{ID: "乙", Wage: 1, Works: []WorkRecord{{StoreID: "A", Minutes: 7}, {StoreID: "C", Minutes: 3}}},
	}
	stores := []Store{{ID: "A", Target: 1}, {ID: "B", Target: 1}, {ID: "C", Target: 0}}
	matrix, ok := Solve(employees, stores)
	if !ok {
		t.Fatal("expected feasible")
	}
	checkMatrix(t, employees, stores, matrix)
	if matrix["甲"]["B"] != 1 || matrix["甲"]["A"] != 0 {
		t.Fatalf("甲 must give 1 cent to B, got %v", matrix["甲"])
	}
	if matrix["乙"]["A"] != 1 || matrix["乙"]["C"] != 0 {
		t.Fatalf("乙 must give 1 cent to A, got %v", matrix["乙"])
	}
}

// TestSolveInfeasibleConnectivity has equal totals and every store target
// within [floor, ceil] of its theoretical column sum, yet stores A and B
// together need 3 cents from only 2 cents of connected wages.
func TestSolveInfeasibleConnectivity(t *testing.T) {
	employees := []Employee{
		{ID: "e1", Wage: 1, Works: []WorkRecord{{StoreID: "A", Minutes: 3}, {StoreID: "B", Minutes: 1}}},
		{ID: "e2", Wage: 1, Works: []WorkRecord{{StoreID: "A", Minutes: 3}, {StoreID: "B", Minutes: 1}}},
		{ID: "e3", Wage: 2, Works: []WorkRecord{{StoreID: "C", Minutes: 3}, {StoreID: "D", Minutes: 1}}},
	}
	stores := []Store{{ID: "A", Target: 2}, {ID: "B", Target: 1}, {ID: "C", Target: 1}, {ID: "D", Target: 0}}
	if _, ok := Solve(employees, stores); ok {
		t.Fatal("expected infeasible")
	}
	if _, ok := bruteForce(employees, stores); ok {
		t.Fatal("brute force disagrees: expected infeasible")
	}
}

func TestSolveAgainstBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(20261003))
	for iter := 0; iter < 400; iter++ {
		numEmp := 1 + rng.Intn(3)
		numStore := 1 + rng.Intn(3)
		empIDs := []string{"e0", "e1", "e2"}
		storeIDs := []string{"s0", "s1", "s2"}
		employees := make([]Employee, numEmp)
		for e := range employees {
			emp := Employee{ID: empIDs[e], Wage: int64(rng.Intn(5))}
			for s := 0; s < numStore; s++ {
				if rng.Float64() < 0.6 {
					emp.Works = append(emp.Works, WorkRecord{StoreID: storeIDs[s], Minutes: 1 + int64(rng.Intn(5))})
				}
			}
			if len(emp.Works) == 0 {
				emp.Works = append(emp.Works, WorkRecord{StoreID: storeIDs[rng.Intn(numStore)], Minutes: 1 + int64(rng.Intn(5))})
			}
			employees[e] = emp
		}
		stores := make([]Store, numStore)
		strategy := rng.Intn(3)
		for s := range stores {
			stores[s].ID = storeIDs[s]
			switch strategy {
			case 0: // arbitrary, often infeasible
				stores[s].Target = int64(rng.Intn(5))
			case 1: // within [floor, ceil] of the theoretical column sum
				var floorSum, fracNum int64
				for _, emp := range employees {
					var total int64
					for _, w := range emp.Works {
						total += w.Minutes
					}
					for _, w := range emp.Works {
						if w.StoreID == storeIDs[s] {
							floorSum += emp.Wage * w.Minutes / total
							fracNum += emp.Wage * w.Minutes % total
						}
					}
				}
				// theory = floorSum + fractional parts; pick target in
				// [floor(theory), ceil(theory)] conservatively.
				lo := floorSum
				hi := floorSum + fracNum // generous upper bound
				if fracNum == 0 {
					hi = floorSum
				}
				stores[s].Target = lo
				if hi > lo {
					stores[s].Target = lo + int64(rng.Intn(int(hi-lo+1)))
				}
			default: // derived from a random rounding, always feasible
				stores[s].Target = 0
			}
		}
		if strategy == 2 {
			for _, emp := range employees {
				var total int64
				for _, w := range emp.Works {
					total += w.Minutes
				}
				var floorSum int64
				floors := make([]int64, len(emp.Works))
				for r, w := range emp.Works {
					floors[r] = emp.Wage * w.Minutes / total
					floorSum += floors[r]
				}
				extra := emp.Wage - floorSum
				order := rng.Perm(len(emp.Works))
				for i, r := range order {
					amt := floors[r]
					if int64(i) < extra {
						amt++
					}
					for s := range stores {
						if stores[s].ID == emp.Works[r].StoreID {
							stores[s].Target += amt
						}
					}
				}
			}
		}

		wantDev, wantOK := bruteForce(employees, stores)
		matrix, ok := Solve(employees, stores)
		if ok != wantOK {
			t.Fatalf("iter %d: feasibility mismatch: Solve=%v brute=%v\nemployees=%+v\nstores=%+v",
				iter, ok, wantOK, employees, stores)
		}
		if !ok {
			continue
		}
		gotDev := checkMatrix(t, employees, stores, matrix)
		if gotDev != wantDev {
			t.Fatalf("iter %d: deviation %d, brute-force optimum %d\nemployees=%+v\nstores=%+v\nmatrix=%v",
				iter, gotDev, wantDev, employees, stores, matrix)
		}
	}
}
