package pow

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestFactors(t *testing.T) {
	p1 := big.NewInt(24801309629)
	p2 := big.NewInt(34244502967)
	input := (new(big.Int).Mul(p1, p2)).String()
	input = strings.TrimSuffix(input, "\n")

	N := new(big.Int)
	N.SetString(input, 10)
	// Start timer
	start := time.Now()
	// Calculation
	factorsList := factors(N)
	// End timer
	elapsed := time.Since(start)
	// Output results
	for _, factor := range factorsList {
		fmt.Println(factor)
	}
	fmt.Printf("Elapsed time: %s\n", elapsed)

	expected := []*big.Int{
		p1,
		p2,
	}

	if !equalSlices(factorsList, expected) {
		t.Errorf("factorsList does not match expected values")
	}
}

func equalSlices(slice1, slice2 []*big.Int) bool {
	if len(slice1) != len(slice2) {
		return false
	}
	for i := range slice1 {
		if slice1[i].Cmp(slice2[i]) != 0 {
			return false
		}
	}
	return true
}
