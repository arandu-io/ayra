package icons

import (
	"fmt"
	"math"
	"strconv"
)

// Op is what a [Segment] does.
type Op uint8

const (
	// MoveTo starts a new contour at P[0].
	MoveTo Op = iota
	// LineTo draws a straight line to P[0].
	LineTo
	// CubicTo draws a cubic curve with the control points P[0] and P[1], ending
	// at P[2]. Quadratic curves and arcs are converted to it, so a drawing layer
	// needs the three operations and no more.
	CubicTo
	// Close ends the contour with a line back to where it started.
	Close
)

// Point is a position on the icon's square, in its own units.
type Point struct{ X, Y float32 }

// Segment is one step of a path, in absolute coordinates.
type Segment struct {
	Op Op
	P  [3]Point
}

// ParsePath reads the path syntax of an SVG path attribute -- M, L, H, V, C, S, Q,
// T, A and Z, absolute and relative -- into absolute segments.
//
// It reads what the icons use and what the specification allows of it, and
// refuses what it cannot read instead of drawing half of it: an icon cut short
// is a different icon, and nobody can tell from the screen which half is
// missing.
func ParsePath(d string) ([]Segment, error) {
	p := parser{src: d}
	return p.parse()
}

type parser struct {
	src string
	at  int

	out        []Segment
	cur, start Point
	// lastControl is the second control point of the previous curve, which a
	// smooth curve (S, T) reflects. It is only meaningful straight after one.
	lastControl Point
	previous    byte
}

func (p *parser) parse() ([]Segment, error) {
	var command byte
	for {
		p.skipSpace()
		if p.at >= len(p.src) {
			return p.out, nil
		}

		if c := p.src[p.at]; isCommand(c) {
			command = c
			p.at++
		} else if command == 0 {
			return nil, fmt.Errorf("icons: the path starts with %q, not a command", c)
		} else if command == 'M' {
			// More pairs after a move are lines.
			command = 'L'
		} else if command == 'm' {
			command = 'l'
		}

		if err := p.run(command); err != nil {
			return nil, err
		}
		p.previous = command
	}
}

func isCommand(c byte) bool {
	switch c {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'S', 's', 'Q', 'q', 'T', 't', 'A', 'a', 'Z', 'z':
		return true
	}
	return false
}

func (p *parser) run(command byte) error {
	relative := command >= 'a'
	base := Point{}
	if relative {
		base = p.cur
	}

	switch command | 0x20 {
	case 'z':
		p.out = append(p.out, Segment{Op: Close})
		p.cur = p.start
		p.previous = 'z'
		return nil

	case 'm', 'l':
		x, y, err := p.pair()
		if err != nil {
			return err
		}
		pt := Point{base.X + x, base.Y + y}
		op := LineTo
		if command|0x20 == 'm' {
			op = MoveTo
			p.start = pt
		}
		p.out = append(p.out, Segment{Op: op, P: [3]Point{pt}})
		p.cur = pt

	case 'h':
		x, err := p.number()
		if err != nil {
			return err
		}
		p.cur.X = base.X + x
		p.out = append(p.out, Segment{Op: LineTo, P: [3]Point{p.cur}})

	case 'v':
		y, err := p.number()
		if err != nil {
			return err
		}
		p.cur.Y = base.Y + y
		p.out = append(p.out, Segment{Op: LineTo, P: [3]Point{p.cur}})

	case 'c', 's':
		var c1, c2, end Point
		if command|0x20 == 'c' {
			x1, y1, err := p.pair()
			if err != nil {
				return err
			}
			c1 = Point{base.X + x1, base.Y + y1}
		} else {
			c1 = p.reflected("cCsS")
		}
		x2, y2, err := p.pair()
		if err != nil {
			return err
		}
		x, y, err := p.pair()
		if err != nil {
			return err
		}
		c2, end = Point{base.X + x2, base.Y + y2}, Point{base.X + x, base.Y + y}
		p.out = append(p.out, Segment{Op: CubicTo, P: [3]Point{c1, c2, end}})
		p.lastControl, p.cur = c2, end

	case 'q', 't':
		var c1 Point
		if command|0x20 == 'q' {
			x1, y1, err := p.pair()
			if err != nil {
				return err
			}
			c1 = Point{base.X + x1, base.Y + y1}
		} else {
			c1 = p.reflected("qQtT")
		}
		x, y, err := p.pair()
		if err != nil {
			return err
		}
		end := Point{base.X + x, base.Y + y}
		// A quadratic curve is the cubic whose control points are two thirds of
		// the way from each end to the one control point.
		a := Point{p.cur.X + 2*(c1.X-p.cur.X)/3, p.cur.Y + 2*(c1.Y-p.cur.Y)/3}
		b := Point{end.X + 2*(c1.X-end.X)/3, end.Y + 2*(c1.Y-end.Y)/3}
		p.out = append(p.out, Segment{Op: CubicTo, P: [3]Point{a, b, end}})
		p.lastControl, p.cur = c1, end

	case 'a':
		rx, err := p.number()
		if err != nil {
			return err
		}
		ry, err := p.number()
		if err != nil {
			return err
		}
		rotation, err := p.number()
		if err != nil {
			return err
		}
		large, err := p.flag()
		if err != nil {
			return err
		}
		sweep, err := p.flag()
		if err != nil {
			return err
		}
		x, y, err := p.pair()
		if err != nil {
			return err
		}
		p.arc(rx, ry, rotation, large, sweep, Point{base.X + x, base.Y + y})
	}
	return nil
}

// reflected is the control point a smooth curve starts with: the previous one
// mirrored through the current point, or the current point itself when the
// previous command was not a curve of the same family.
func (p *parser) reflected(family string) Point {
	for i := 0; i < len(family); i++ {
		if p.previous == family[i] {
			return Point{2*p.cur.X - p.lastControl.X, 2*p.cur.Y - p.lastControl.Y}
		}
	}
	return p.cur
}

func (p *parser) skipSpace() {
	for p.at < len(p.src) && (p.src[p.at] == ' ' || p.src[p.at] == ',' || p.src[p.at] == '\n' || p.src[p.at] == '\t' || p.src[p.at] == '\r') {
		p.at++
	}
}

func (p *parser) pair() (x, y float32, err error) {
	if x, err = p.number(); err != nil {
		return
	}
	y, err = p.number()
	return
}

// flag reads an arc flag, which is one character and may be written with no
// separator after it: "a1 1 0 011 1" is two flags and then the number 1.
func (p *parser) flag() (bool, error) {
	p.skipSpace()
	if p.at >= len(p.src) || (p.src[p.at] != '0' && p.src[p.at] != '1') {
		return false, fmt.Errorf("icons: an arc flag is 0 or 1, at %d", p.at)
	}
	on := p.src[p.at] == '1'
	p.at++
	return on, nil
}

func (p *parser) number() (float32, error) {
	p.skipSpace()
	start := p.at
	if p.at < len(p.src) && (p.src[p.at] == '+' || p.src[p.at] == '-') {
		p.at++
	}
	seenDot := false
	for p.at < len(p.src) {
		c := p.src[p.at]
		switch {
		case c >= '0' && c <= '9':
			p.at++
			continue
		case c == '.' && !seenDot:
			// A second dot starts the next number: ".5.5" is two of them.
			seenDot = true
			p.at++
			continue
		case (c == 'e' || c == 'E') && p.at > start:
			p.at++
			if p.at < len(p.src) && (p.src[p.at] == '+' || p.src[p.at] == '-') {
				p.at++
			}
			continue
		}
		break
	}
	v, err := strconv.ParseFloat(p.src[start:p.at], 32)
	if err != nil {
		return 0, fmt.Errorf("icons: a number was expected at %d: %w", start, err)
	}
	return float32(v), nil
}

// arc appends the cubic curves that approximate an elliptical arc, by the
// conversion from endpoint to centre parameters the SVG specification gives.
func (p *parser) arc(rx, ry, rotation float32, large, sweep bool, end Point) {
	start := p.cur
	p.cur = end
	if start == end {
		return
	}
	rx, ry = abs32(rx), abs32(ry)
	if rx == 0 || ry == 0 {
		p.out = append(p.out, Segment{Op: LineTo, P: [3]Point{end}})
		return
	}

	phi := float64(rotation) * math.Pi / 180
	cos, sin := math.Cos(phi), math.Sin(phi)
	dx, dy := float64(start.X-end.X)/2, float64(start.Y-end.Y)/2
	x1, y1 := cos*dx+sin*dy, -sin*dx+cos*dy

	rxf, ryf := float64(rx), float64(ry)
	if lambda := x1*x1/(rxf*rxf) + y1*y1/(ryf*ryf); lambda > 1 {
		scale := math.Sqrt(lambda)
		rxf, ryf = rxf*scale, ryf*scale
	}

	numerator := rxf*rxf*ryf*ryf - rxf*rxf*y1*y1 - ryf*ryf*x1*x1
	denominator := rxf*rxf*y1*y1 + ryf*ryf*x1*x1
	coefficient := 0.0
	if denominator != 0 && numerator > 0 {
		coefficient = math.Sqrt(numerator / denominator)
	}
	if large == sweep {
		coefficient = -coefficient
	}
	cx1, cy1 := coefficient*rxf*y1/ryf, -coefficient*ryf*x1/rxf
	cx := cos*cx1 - sin*cy1 + float64(start.X+end.X)/2
	cy := sin*cx1 + cos*cy1 + float64(start.Y+end.Y)/2

	angle := func(ux, uy, vx, vy float64) float64 {
		a := math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy)
		return a
	}
	theta := angle(1, 0, (x1-cx1)/rxf, (y1-cy1)/ryf)
	delta := angle((x1-cx1)/rxf, (y1-cy1)/ryf, (-x1-cx1)/rxf, (-y1-cy1)/ryf)
	if !sweep && delta > 0 {
		delta -= 2 * math.Pi
	} else if sweep && delta < 0 {
		delta += 2 * math.Pi
	}

	// One cubic per quarter turn at most keeps the error under a hundredth of a
	// unit, which on an icon is a fraction of a pixel.
	pieces := int(math.Ceil(math.Abs(delta) / (math.Pi / 2)))
	step := delta / float64(pieces)
	t := 4.0 / 3.0 * math.Tan(step/4)

	point := func(a float64) (float64, float64) {
		return cx + rxf*math.Cos(a)*cos - ryf*math.Sin(a)*sin, cy + rxf*math.Cos(a)*sin + ryf*math.Sin(a)*cos
	}
	derivative := func(a float64) (float64, float64) {
		return -rxf*math.Sin(a)*cos - ryf*math.Cos(a)*sin, -rxf*math.Sin(a)*sin + ryf*math.Cos(a)*cos
	}

	for i := 0; i < pieces; i++ {
		a0, a1 := theta+float64(i)*step, theta+float64(i+1)*step
		x0, y0 := point(a0)
		xe, ye := point(a1)
		d0x, d0y := derivative(a0)
		d1x, d1y := derivative(a1)
		last := Point{float32(xe), float32(ye)}
		if i == pieces-1 {
			last = end
		}
		p.out = append(p.out, Segment{Op: CubicTo, P: [3]Point{
			{float32(x0 + t*d0x), float32(y0 + t*d0y)},
			{float32(xe - t*d1x), float32(ye - t*d1y)},
			last,
		}})
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
