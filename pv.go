package main

import (
	"fmt"
	"math"
	"strconv"

	ga "saml.dev/gome-assistant"
)

const maxWatt = 2000.0

type lightController interface {
	TurnOn(entityId string, serviceData ...map[string]any) error
}

type stateGetter interface {
	Get(entityId string) (ga.EntityState, error)
}

func hslToRgb(h, s, l float64) [3]int {
	hue2rgb := func(p, q, t float64) float64 {
		if t < 0 {
			t += 1
		}
		if t > 1 {
			t -= 1
		}
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 1.0/2:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		default:
			return p
		}
	}

	var r, g, b float64
	if s == 0 {
		r, g, b = l, l, l
	} else {
		var q float64
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q
		r = hue2rgb(p, q, h+1.0/3)
		g = hue2rgb(p, q, h)
		b = hue2rgb(p, q, h-1.0/3)
	}
	return [3]int{int(math.Floor(r * 255)), int(math.Floor(g * 255)), int(math.Floor(b * 255))}
}

func wattToRGB(watts float64) [3]int {
	hue := (math.Min(watts, maxWatt) / maxWatt * 100) * 1.2 / 360
	return hslToRgb(hue, 1, 0.5)
}

// applyPVColor sets lightID's color proportional to feed-in power,
// but only when zoneID reports someone home. watts <= 0 is a no-op.
func applyPVColor(light lightController, state stateGetter, lightID, zoneID string, watts float64) error {
	if watts <= 0 {
		return nil
	}

	homeState, err := state.Get(zoneID)
	if err != nil {
		return fmt.Errorf("getting zone state: %w", err)
	}
	count, err := strconv.ParseFloat(homeState.State, 64)
	if err != nil || count <= 0 {
		return nil
	}

	rgb := wattToRGB(watts)
	return light.TurnOn(lightID, map[string]any{
		"rgb_color":      []int{rgb[0], rgb[1], rgb[2]},
		"brightness_pct": 1,
	})
}
