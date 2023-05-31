package pow

import (
	"math/big"
)

func gcd(a, b *big.Int) *big.Int {
	return new(big.Int).GCD(nil, nil, a, b)
}

func abs(a *big.Int) *big.Int {
	return new(big.Int).Abs(a)
}

func rho(N *big.Int) *big.Int {
	two := big.NewInt(2)
	if new(big.Int).Mod(N, two).Cmp(big.NewInt(0)) == 0 {
		return two
	}

	x := big.NewInt(0).Set(N)
	y := big.NewInt(0).Set(N)
	c := big.NewInt(1)
	g := big.NewInt(1)

	for g.Cmp(big.NewInt(1)) == 0 {
		x.Mul(x, x).Add(x, c).Mod(x, N)
		y.Mul(y, y).Add(y, c).Mod(y, N)
		y.Mul(y, y).Add(y, c).Mod(y, N)
		g = gcd(abs(new(big.Int).Sub(x, y)), N)
	}
	return g
}

func factors(N *big.Int) []*big.Int {
	one := big.NewInt(1)
	if N.Cmp(one) == 0 {
		return []*big.Int{}
	}
	factor := rho(N)
	if factor.Cmp(N) == 0 {
		return []*big.Int{N}
	}
	return append(factors(factor), factors(new(big.Int).Div(N, factor))...)
}
