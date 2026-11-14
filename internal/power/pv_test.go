package power

import (
	"math"
	"testing"

	"github.com/LlvallVeiz/powerorbit/internal/config"
	"github.com/LlvallVeiz/powerorbit/internal/orbit"
)

// approx reports whether got is within tol of want.
func approx(t *testing.T, what string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.4f, want %.4f ± %.4f", what, got, want, tol)
	}
}

// referencePanel loads the 1U reference mission and builds its panel:
// 0.01 m², 30% GaAs cells, 90% MPPT, 2%/year degradation.
func referencePanel(t *testing.T) Panel {
	t.Helper()
	m, err := config.Load("../../configs/1u-reference.yaml")
	if err != nil {
		t.Fatalf("loading 1u-reference.yaml: %v", err)
	}
	p, err := NewPanel(m.SolarPanel)
	if err != nil {
		t.Fatalf("NewPanel(1U reference): %v", err)
	}
	return p
}

func TestPanelBOLPowerMatchesDay2(t *testing.T) {
	// Day 2's config note: the 1U panel is 4.083 W raw at normal
	// incidence in sunlight (1361 × 0.01 × 0.30), 3.675 W after MPPT.
	p := referencePanel(t)
	raw, err := p.RawPowerW(1, 1, 0)
	if err != nil {
		t.Fatalf("RawPowerW: %v", err)
	}
	approx(t, "BOL raw power (W)", raw, 4.083, 0.001)
	power, err := p.PowerW(1, 1, 0)
	if err != nil {
		t.Fatalf("PowerW: %v", err)
	}
	approx(t, "BOL power after MPPT (W)", power, 3.6747, 0.001)
	approx(t, "peak power (W)", p.PeakPowerW(), 3.6747, 0.001)
}

func TestPanelOrbitAverage500KmBeta0(t *testing.T) {
	// 500 km, beta = 0°: sunlight fraction (94 min 37 s − 35 min 45 s) /
	// 94 min 37 s = 0.6222, so the orbit-average power is 4.083 × 0.6222
	// = 2.540 W raw, 2.286 W after MPPT. The fraction comes from the Day
	// 3 orbit engine, not from a hardcoded number.
	p := referencePanel(t)
	o := orbit.CircularOrbit{AltitudeKm: 500, BetaDeg: 0}
	approx(t, "sunlit fraction at 500 km, beta 0", o.SunlitFraction(), 0.6222, 0.001)
	avg, err := p.OrbitAveragePowerW(o, 1, 0)
	if err != nil {
		t.Fatalf("OrbitAveragePowerW: %v", err)
	}
	approx(t, "orbit-average power after MPPT (W)", avg, 2.286, 0.005)
	// The same average from the raw power scaled by the sunlit fraction.
	raw, err := p.RawPowerW(1, 1, 0)
	if err != nil {
		t.Fatalf("RawPowerW: %v", err)
	}
	approx(t, "orbit-average raw power (W)", raw*o.SunlitFraction(), 2.540, 0.005)
}

func TestPanelDegradationAfterOneYear(t *testing.T) {
	// 2%/year: after 1 year the BOL 4.083 W is 4.083 × 0.98 = 4.001 W.
	p := referencePanel(t)
	factor, err := p.DegradationFactor(1)
	if err != nil {
		t.Fatalf("DegradationFactor: %v", err)
	}
	approx(t, "degradation factor after 1 year", factor, 0.98, 1e-12)
	raw, err := p.RawPowerW(1, 1, 1)
	if err != nil {
		t.Fatalf("RawPowerW: %v", err)
	}
	approx(t, "raw power after 1 year (W)", raw, 4.001, 0.001)
	power, err := p.PowerW(1, 1, 1)
	if err != nil {
		t.Fatalf("PowerW: %v", err)
	}
	approx(t, "power after MPPT after 1 year (W)", power, 3.601, 0.001)
}

func TestPanelDegradationCompounds(t *testing.T) {
	// Degradation compounds: after 5 years the factor is 0.98^5, not
	// 1 − 5×0.02.
	p := referencePanel(t)
	factor, err := p.DegradationFactor(5)
	if err != nil {
		t.Fatalf("DegradationFactor: %v", err)
	}
	approx(t, "degradation factor after 5 years", factor, math.Pow(0.98, 5), 1e-12)
	if factor <= 0.90 {
		t.Errorf("5-year factor = %.4f, expected 0.98^5 ≈ 0.9039 (above the linear 1 − 5×0.02 = 0.90)", factor)
	}
	// Beginning of life: no degradation yet.
	factor, err = p.DegradationFactor(0)
	if err != nil {
		t.Fatalf("DegradationFactor: %v", err)
	}
	approx(t, "degradation factor at BOL", factor, 1.0, 1e-12)
}

func TestPanelCosineHalvesPower(t *testing.T) {
	// Incidence at 60° (cos θ = 0.5) halves the power: the cosine law is
	// the whole point of the incidence term.
	p := referencePanel(t)
	full, err := p.PowerW(1, 1, 0)
	if err != nil {
		t.Fatalf("PowerW: %v", err)
	}
	half, err := p.PowerW(0.5, 1, 0)
	if err != nil {
		t.Fatalf("PowerW: %v", err)
	}
	approx(t, "power at cos(theta) = 0.5 (W)", half, full/2, 1e-9)
	approx(t, "power at cos(theta) = 0.5 (W)", half, 1.837, 0.001)
}

func TestPanelEclipseGivesZero(t *testing.T) {
	// In eclipse the orbit engine reports SunFactor 0 — the panel
	// produces nothing, however good the incidence angle.
	p := referencePanel(t)
	power, err := p.PowerW(1, 0, 0)
	if err != nil {
		t.Fatalf("PowerW: %v", err)
	}
	if power != 0 {
		t.Errorf("power in eclipse = %g W, want 0", power)
	}
}

func TestPanelFacingAwayGivesZero(t *testing.T) {
	// Negative cosine: the panel faces away from the Sun. Clamped to 0
	// rather than producing negative power.
	p := referencePanel(t)
	power, err := p.PowerW(-0.5, 1, 0)
	if err != nil {
		t.Fatalf("PowerW: %v", err)
	}
	if power != 0 {
		t.Errorf("power facing away = %g W, want 0", power)
	}
}

func TestPanelValidation(t *testing.T) {
	cases := []struct {
		name string
		p    Panel
	}{
		{"zero area", Panel{AreaM2: 0, Efficiency: 0.30, MPPTefficiency: 0.90, DegradationPerYear: 0.02}},
		{"negative area", Panel{AreaM2: -0.01, Efficiency: 0.30, MPPTefficiency: 0.90, DegradationPerYear: 0.02}},
		{"zero efficiency", Panel{AreaM2: 0.01, Efficiency: 0, MPPTefficiency: 0.90, DegradationPerYear: 0.02}},
		{"efficiency above 1", Panel{AreaM2: 0.01, Efficiency: 1.2, MPPTefficiency: 0.90, DegradationPerYear: 0.02}},
		{"zero MPPT efficiency", Panel{AreaM2: 0.01, Efficiency: 0.30, MPPTefficiency: 0, DegradationPerYear: 0.02}},
		{"MPPT efficiency above 1", Panel{AreaM2: 0.01, Efficiency: 0.30, MPPTefficiency: 1.1, DegradationPerYear: 0.02}},
		{"negative degradation", Panel{AreaM2: 0.01, Efficiency: 0.30, MPPTefficiency: 0.90, DegradationPerYear: -0.01}},
		{"degradation above 50%/year", Panel{AreaM2: 0.01, Efficiency: 0.30, MPPTefficiency: 0.90, DegradationPerYear: 0.6}},
	}
	for _, c := range cases {
		if err := c.p.Validate(); err == nil {
			t.Errorf("Validate(%s) = nil, want an error", c.name)
		}
		if _, err := NewPanel(config.SolarPanel{
			AreaM2:             c.p.AreaM2,
			Efficiency:         c.p.Efficiency,
			MPPTefficiency:     c.p.MPPTefficiency,
			DegradationPerYear: c.p.DegradationPerYear,
		}); err == nil {
			t.Errorf("NewPanel(%s) = nil error, want an error", c.name)
		}
	}
	// The reference panel itself is valid.
	if err := referencePanel(t).Validate(); err != nil {
		t.Errorf("Validate(1U reference) = %v, want nil", err)
	}
}

func TestPanelRejectsImpossibleInputs(t *testing.T) {
	p := referencePanel(t)
	if _, err := p.PowerW(1.5, 1, 0); err == nil {
		t.Errorf("PowerW(cos > 1) = nil error, want an error")
	}
	if _, err := p.PowerW(1, 1.5, 0); err == nil {
		t.Errorf("PowerW(sun factor > 1) = nil error, want an error")
	}
	if _, err := p.PowerW(1, -0.1, 0); err == nil {
		t.Errorf("PowerW(sun factor < 0) = nil error, want an error")
	}
	if _, err := p.PowerW(1, 1, -1); err == nil {
		t.Errorf("PowerW(negative mission age) = nil error, want an error")
	}
	if _, err := p.OrbitAveragePowerW(orbit.CircularOrbit{AltitudeKm: 500, BetaDeg: 0}, 2, 0); err == nil {
		t.Errorf("OrbitAveragePowerW(cos > 1) = nil error, want an error")
	}
}

func TestPanelLargerArrayScalesLinearly(t *testing.T) {
	// Power scales linearly with area: a 3U face (0.03 m²) gives three
	// times the 1U panel at the same efficiency.
	p := referencePanel(t)
	oneU, err := p.PowerW(1, 1, 0)
	if err != nil {
		t.Fatalf("PowerW: %v", err)
	}
	big, err := NewPanel(config.SolarPanel{AreaM2: 0.03, Efficiency: 0.30, MPPTefficiency: 0.90, DegradationPerYear: 0.02})
	if err != nil {
		t.Fatalf("NewPanel(3U): %v", err)
	}
	threeU, err := big.PowerW(1, 1, 0)
	if err != nil {
		t.Fatalf("PowerW: %v", err)
	}
	approx(t, "3U panel power (W)", threeU, 3*oneU, 1e-9)
}
