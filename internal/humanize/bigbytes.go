package humanize

import (
	"fmt"
	"math/big"
)

var (
	bigSIExp  = big.NewInt(1000)
	bigIECExp = big.NewInt(1024)
)

// BigBytes produces a human-readable representation of an SI size.
func BigBytes(s *big.Int, precision int) string {
	sizes := []string{"B", "kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	return humanizeBigBytes(s, bigSIExp, sizes, precision)
}

// BigIBytes produces a human-readable representation of an IEC size.
func BigIBytes(s *big.Int, precision int) string {
	sizes := []string{"B", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi"}
	return humanizeBigBytes(s, bigIECExp, sizes, precision)
}

var ten = big.NewInt(10)

func humanizeBigBytes(s, base *big.Int, sizes []string, precision int) string {
	if s.Cmp(ten) < 0 {
		return fmt.Sprintf("%d B", s)
	}
	c := (&big.Int{}).Set(s)
	val, mag := orderOfMagnitude(c, base, len(sizes)-1)
	suffix := sizes[mag]
	f := "%.0f %s"
	if val < 10 {
		f = fmt.Sprintf("%%.%df %%s", precision)
	}
	return fmt.Sprintf(f, val, suffix)
}

func orderOfMagnitude(n, b *big.Int, max int) (float64, int) {
	mag := 0
	m := &big.Int{}
	for n.Cmp(b) >= 0 {
		n.DivMod(n, b, m)
		mag++
		if mag == max && max >= 0 {
			break
		}
	}
	return float64(n.Int64()) + (float64(m.Int64()) / float64(b.Int64())), mag
}
