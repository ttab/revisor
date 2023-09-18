package revisor

import (
	"fmt"
	"strings"

	"github.com/IvanZagoskin/wkt/geometry"
	"github.com/IvanZagoskin/wkt/parser"
)

type Geometry string

const (
	GeometryAny             Geometry = ""
	GeometryPoint           Geometry = "point"
	GeometryMultiPoint      Geometry = "multipoint"
	GeometryLineString      Geometry = "linestring"
	GeometryMultiLineString Geometry = "multilinestring"
	GeometryPolygon         Geometry = "polygon"
	GeometryMultiPolygon    Geometry = "multipolygon"
	GeometryCircularString  Geometry = "circularstring"
)

var coords = map[string]geometry.CoordinateType{
	"":   geometry.XY,
	"z":  geometry.XYZ,
	"m":  geometry.XYM,
	"zm": geometry.XYZM,
}

func errCoordMismatch(want string, got geometry.CoordinateType) error {
	var gotName string

	for k, t := range coords {
		if t == got {
			gotName = k
			break
		}
	}

	if want == "" {
		return fmt.Errorf(
			"unexpected coordinate type %q where none was expected",
			gotName)
	}

	if got == geometry.XY {
		return fmt.Errorf(
			"missing coordinate type where %q was expected",
			want)
	}

	return fmt.Errorf(
		"unexpected coordinate type %q where %q was expected",
		gotName, want)
}

func validateWKT(spec string, value string) error {
	parser := parser.New()

	geo, err := parser.ParseWKT(strings.NewReader(value))
	if err != nil {
		return fmt.Errorf("failed to parse: %w", err)
	}

	g, coord, _ := strings.Cut(spec, "-")

	ct, ok := coords[coord]
	if !ok {
		return fmt.Errorf("unknown coordinate type %q", coord)
	}

	switch Geometry(g) {
	case GeometryAny:
	case GeometryPoint:
		p, ok := geo.(*geometry.Point)
		if !ok {
			return fmt.Errorf("geometry is not a point")
		}

		if p.Type != ct {
			return errCoordMismatch(coord, p.Type)
		}
	case GeometryLineString:
		l, ok := geo.(*geometry.LineString)
		if !ok {
			return fmt.Errorf("geometry is not a linestring")
		}

		if l.Type != ct {
			return errCoordMismatch(coord, l.Type)
		}
	case GeometryPolygon:
		p, ok := geo.(*geometry.Polygon)
		if !ok {
			return fmt.Errorf("geometry is not a polygon")
		}

		for _, l := range p.LineStrings {
			if l.Type != ct {
				return errCoordMismatch(coord, l.Type)
			}
		}
	case GeometryMultiPoint:
		mp, ok := geo.(*geometry.MultiPoint)
		if !ok {
			return fmt.Errorf("geometry is not a multipoint")
		}

		for _, p := range mp.Points {
			if p.Type != ct {
				return errCoordMismatch(coord, p.Type)
			}
		}
	case GeometryMultiLineString:
		ml, ok := geo.(*geometry.MultiLineString)
		if !ok {
			return fmt.Errorf("geometry is not a multilinestring")
		}

		for _, l := range ml.Lines {
			if l.Type != ct {
				return errCoordMismatch(coord, l.Type)
			}
		}
	case GeometryMultiPolygon:
		mp, ok := geo.(*geometry.MultiPolygon)
		if !ok {
			return fmt.Errorf("geometry is not a multipolygon")
		}

		for _, p := range mp.Polygons {
			for _, l := range p.LineStrings {
				if l.Type != ct {
					return errCoordMismatch(coord, l.Type)
				}
			}
		}
	case GeometryCircularString:
		cs, ok := geo.(*geometry.CircularString)
		if !ok {
			return fmt.Errorf("geometry is not a circular string")
		}

		for _, l := range cs.Points {
			if l.Type != ct {
				return errCoordMismatch(coord, l.Type)
			}
		}
	default:
		return fmt.Errorf("unknown geometry type %q", g)
	}

	return nil
}
