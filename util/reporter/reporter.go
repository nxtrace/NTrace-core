package reporter

import (
	"fmt"

	"github.com/xgadget-lab/nexttrace/methods"
)

type Reporter interface {
	Print()
}

func New(rs map[uint16][]methods.TracerouteHop) Reporter {
	r := reporter{
		routeResult: rs,
	}
	fmt.Println(r)
	return &r
}

type reporter struct {
	routeResult map[uint16][]methods.TracerouteHop
}

func (r *reporter) Print() {
	fmt.Println(r)
}
