package trayicon

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

type point struct{ x, y float64 }

func (p point) add(q point) point { return point{p.x + q.x, p.y + q.y} }

const flattenTol = 0.35

func parsePath(d string) ([][]point, error) {
	tokens := lexPath(d)
	var (
		contours [][]point
		cur      []point
		x, y     float64
		sx, sy   float64
		cx, cy   float64
		qx, qy   float64
		last     byte
		i        int
	)
	flush := func() {
		if len(cur) >= 3 {
			contours = append(contours, cur)
		}
		cur = nil
	}
	moveTo := func(nx, ny float64) {
		flush()
		x, y = nx, ny
		sx, sy = nx, ny
		cur = []point{{nx, ny}}
		cx, cy = nx, ny
		qx, qy = nx, ny
	}
	lineTo := func(nx, ny float64) {
		if cur == nil {
			moveTo(x, y)
		}
		x, y = nx, ny
		cur = append(cur, point{nx, ny})
		cx, cy = nx, ny
		qx, qy = nx, ny
	}

	for i < len(tokens) {
		tok := tokens[i]
		if len(tok) != 1 || !isCmd(tok[0]) {
			return nil, fmt.Errorf("svg path: expected command, got %q", tok)
		}
		cmd := tok[0]
		i++
		rel := unicode.IsLower(rune(cmd))
		kind := byte(unicode.ToUpper(rune(cmd)))

		take := func(n int) ([]float64, error) {
			if i+n > len(tokens) {
				return nil, fmt.Errorf("svg path: %c needs %d numbers", cmd, n)
			}
			out := make([]float64, n)
			for k := 0; k < n; k++ {
				if isCmd(tokens[i][0]) {
					return nil, fmt.Errorf("svg path: %c short argument list", cmd)
				}
				v, err := parseNum(tokens[i])
				if err != nil {
					return nil, err
				}
				out[k] = v
				i++
			}
			return out, nil
		}

		repeat := true
		first := true
		for repeat {
			if !first {
				if i >= len(tokens) || isCmd(tokens[i][0]) {
					break
				}
				if kind == 'M' {
					kind = 'L'
					if rel {
						cmd = 'l'
					} else {
						cmd = 'L'
					}
				}
			}
			first = false
			switch kind {
			case 'M':
				n, err := take(2)
				if err != nil {
					return nil, err
				}
				nx, ny := n[0], n[1]
				if rel {
					nx += x
					ny += y
				}
				moveTo(nx, ny)
			case 'L':
				n, err := take(2)
				if err != nil {
					return nil, err
				}
				nx, ny := n[0], n[1]
				if rel {
					nx += x
					ny += y
				}
				lineTo(nx, ny)
			case 'H':
				n, err := take(1)
				if err != nil {
					return nil, err
				}
				nx := n[0]
				if rel {
					nx += x
				}
				lineTo(nx, y)
			case 'V':
				n, err := take(1)
				if err != nil {
					return nil, err
				}
				ny := n[0]
				if rel {
					ny += y
				}
				lineTo(x, ny)
			case 'C':
				n, err := take(6)
				if err != nil {
					return nil, err
				}
				x1, y1, x2, y2, nx, ny := n[0], n[1], n[2], n[3], n[4], n[5]
				if rel {
					x1 += x
					y1 += y
					x2 += x
					y2 += y
					nx += x
					ny += y
				}
				cur = append(cur, flattenCubic(point{x, y}, point{x1, y1}, point{x2, y2}, point{nx, ny})[1:]...)
				cx, cy = x2, y2
				qx, qy = nx, ny
				x, y = nx, ny
			case 'S':
				n, err := take(4)
				if err != nil {
					return nil, err
				}
				x2, y2, nx, ny := n[0], n[1], n[2], n[3]
				if rel {
					x2 += x
					y2 += y
					nx += x
					ny += y
				}
				x1, y1 := x, y
				if last == 'C' || last == 'S' || last == 'c' || last == 's' {
					x1, y1 = 2*x-cx, 2*y-cy
				}
				cur = append(cur, flattenCubic(point{x, y}, point{x1, y1}, point{x2, y2}, point{nx, ny})[1:]...)
				cx, cy = x2, y2
				qx, qy = nx, ny
				x, y = nx, ny
			case 'Q':
				n, err := take(4)
				if err != nil {
					return nil, err
				}
				x1, y1, nx, ny := n[0], n[1], n[2], n[3]
				if rel {
					x1 += x
					y1 += y
					nx += x
					ny += y
				}
				cur = append(cur, flattenQuad(point{x, y}, point{x1, y1}, point{nx, ny})[1:]...)
				qx, qy = x1, y1
				cx, cy = nx, ny
				x, y = nx, ny
			case 'T':
				n, err := take(2)
				if err != nil {
					return nil, err
				}
				nx, ny := n[0], n[1]
				if rel {
					nx += x
					ny += y
				}
				x1, y1 := x, y
				if last == 'Q' || last == 'T' || last == 'q' || last == 't' {
					x1, y1 = 2*x-qx, 2*y-qy
				}
				cur = append(cur, flattenQuad(point{x, y}, point{x1, y1}, point{nx, ny})[1:]...)
				qx, qy = x1, y1
				cx, cy = nx, ny
				x, y = nx, ny
			case 'A':
				n, err := take(7)
				if err != nil {
					return nil, err
				}
				rx, ry, phi := math.Abs(n[0]), math.Abs(n[1]), n[2]
				large, sweep := n[3] != 0, n[4] != 0
				nx, ny := n[5], n[6]
				if rel {
					nx += x
					ny += y
				}
				pts := flattenArc(x, y, rx, ry, phi, large, sweep, nx, ny)
				if len(pts) > 1 {
					cur = append(cur, pts[1:]...)
				}
				x, y = nx, ny
				cx, cy = nx, ny
				qx, qy = nx, ny
			case 'Z':
				lineTo(sx, sy)
				flush()
				x, y = sx, sy
				repeat = false
			default:
				return nil, fmt.Errorf("svg path: unsupported %c", cmd)
			}
			last = cmd
			if kind == 'Z' {
				break
			}
		}
	}
	flush()
	if len(contours) == 0 {
		return nil, fmt.Errorf("svg path: no contours")
	}
	return contours, nil
}

func isCmd(c byte) bool {
	switch c {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'S', 's', 'Q', 'q', 'T', 't', 'A', 'a', 'Z', 'z':
		return true
	}
	return false
}

func lexPath(d string) []string {
	d = strings.TrimSpace(d)
	var tokens []string
	i := 0
	for i < len(d) {
		for i < len(d) && (d[i] == ' ' || d[i] == ',' || d[i] == '\t' || d[i] == '\n' || d[i] == '\r') {
			i++
		}
		if i >= len(d) {
			break
		}
		if isCmd(d[i]) {
			tokens = append(tokens, d[i:i+1])
			i++
			continue
		}
		start := i
		if d[i] == '+' || d[i] == '-' {
			i++
		}
		sawDot := false
		sawExp := false
		for i < len(d) {
			c := d[i]
			if c >= '0' && c <= '9' {
				i++
				continue
			}
			if c == '.' && !sawDot && !sawExp {
				sawDot = true
				i++
				continue
			}
			if (c == 'e' || c == 'E') && !sawExp {
				sawExp = true
				i++
				if i < len(d) && (d[i] == '+' || d[i] == '-') {
					i++
				}
				continue
			}
			break
		}
		if i > start {
			tokens = append(tokens, d[start:i])
		} else {
			i++
		}
	}
	return tokens
}

func parseNum(s string) (float64, error) {
	var v float64
	n, err := fmt.Sscanf(s, "%f", &v)
	if err != nil || n != 1 {
		return 0, fmt.Errorf("svg path number %q", s)
	}
	return v, nil
}

func flattenCubic(p0, p1, p2, p3 point) []point {
	if cubicFlat(p0, p1, p2, p3) {
		return []point{p0, p3}
	}
	p01 := mid(p0, p1)
	p12 := mid(p1, p2)
	p23 := mid(p2, p3)
	p012 := mid(p01, p12)
	p123 := mid(p12, p23)
	p0123 := mid(p012, p123)
	left := flattenCubic(p0, p01, p012, p0123)
	right := flattenCubic(p0123, p123, p23, p3)
	return append(left, right[1:]...)
}

func cubicFlat(p0, p1, p2, p3 point) bool {
	return distToSeg(p1, p0, p3) <= flattenTol && distToSeg(p2, p0, p3) <= flattenTol
}

func flattenQuad(p0, p1, p2 point) []point {
	c1 := point{p0.x + 2.0/3.0*(p1.x-p0.x), p0.y + 2.0/3.0*(p1.y-p0.y)}
	c2 := point{p2.x + 2.0/3.0*(p1.x-p2.x), p2.y + 2.0/3.0*(p1.y-p2.y)}
	return flattenCubic(p0, c1, c2, p2)
}

func flattenArc(x1, y1, rx, ry, phiDeg float64, large, sweep bool, x2, y2 float64) []point {
	if rx == 0 || ry == 0 || (x1 == x2 && y1 == y2) {
		return []point{{x1, y1}, {x2, y2}}
	}
	phi := phiDeg * math.Pi / 180
	cosφ, sinφ := math.Cos(phi), math.Sin(phi)
	dx := (x1 - x2) / 2
	dy := (y1 - y2) / 2
	x1p := cosφ*dx + sinφ*dy
	y1p := -sinφ*dx + cosφ*dy
	lam := (x1p*x1p)/(rx*rx) + (y1p*y1p)/(ry*ry)
	if lam > 1 {
		s := math.Sqrt(lam)
		rx *= s
		ry *= s
	}
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	num := rx*rx*ry*ry - rx*rx*y1p*y1p - ry*ry*x1p*x1p
	if den == 0 {
		return []point{{x1, y1}, {x2, y2}}
	}
	coef := math.Sqrt(math.Max(0, num/den))
	if large == sweep {
		coef = -coef
	}
	cxp := coef * rx * y1p / ry
	cyp := coef * -ry * x1p / rx
	cx := cosφ*cxp - sinφ*cyp + (x1+x2)/2
	cy := sinφ*cxp + cosφ*cyp + (y1+y2)/2
	theta1 := vecAngle(1, 0, (x1p-cxp)/rx, (y1p-cyp)/ry)
	dtheta := vecAngle((x1p-cxp)/rx, (y1p-cyp)/ry, (-x1p-cxp)/rx, (-y1p-cyp)/ry)
	if !sweep && dtheta > 0 {
		dtheta -= 2 * math.Pi
	}
	if sweep && dtheta < 0 {
		dtheta += 2 * math.Pi
	}
	steps := int(math.Ceil(math.Abs(dtheta) / (math.Pi / 12)))
	if steps < 1 {
		steps = 1
	}
	out := make([]point, 0, steps+1)
	out = append(out, point{x1, y1})
	for s := 1; s <= steps; s++ {
		th := theta1 + dtheta*float64(s)/float64(steps)
		px := cx + rx*math.Cos(th)*cosφ - ry*math.Sin(th)*sinφ
		py := cy + rx*math.Cos(th)*sinφ + ry*math.Sin(th)*cosφ
		out = append(out, point{px, py})
	}
	out[len(out)-1] = point{x2, y2}
	return out
}

func vecAngle(ux, uy, vx, vy float64) float64 {
	dot := ux*vx + uy*vy
	mod := math.Sqrt((ux*ux + uy*uy) * (vx*vx + vy*vy))
	if mod == 0 {
		return 0
	}
	c := math.Min(1, math.Max(-1, dot/mod))
	ang := math.Acos(c)
	if ux*vy-uy*vx < 0 {
		return -ang
	}
	return ang
}

func mid(a, b point) point { return point{(a.x + b.x) / 2, (a.y + b.y) / 2} }

func distToSeg(p, a, b point) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	if dx == 0 && dy == 0 {
		return math.Hypot(p.x-a.x, p.y-a.y)
	}
	t := ((p.x-a.x)*dx + (p.y-a.y)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return math.Hypot(p.x-a.x-t*dx, p.y-a.y-t*dy)
}
