// Package power models the photovoltaic (PV) generation side of a CubeSat
// electrical power system: how much power the solar array produces at any
// moment of an orbit.
//
// The model is the one in the project plan (rencana-powerorbit.md §5.3):
//
//	P = S · A · η · cos θ · SunFactor · η_mppt · (1 − d)^years
//
// where S = 1361 W/m² is the solar constant, A the illuminated panel area,
// η the cell efficiency, θ the Sun incidence angle on the panel, SunFactor
// the illumination from the orbit engine (1 in sunlight, 0 in eclipse),
// η_mppt the MPPT/EPS path efficiency, and d the fractional degradation
// per year of mission life.
//
// Every constant has a literature anchor (plan §2):
//
//   - Solar constant 1361 W/m² (Artemis CubeSat Kit, Univ. Hawaii).
//   - Triple-junction GaAs cells > 30% efficient (Springer CubeSat-IoT
//     paper) — the 1U reference config uses η = 0.30.
//   - Degradation 2%/year is the planning value in the 1U reference
//     config; it is configurable, not a measured property of a specific
//     array.
//
// Deliberate simplifications (v0.1 estimator, not a flight model): the
// cells are assumed isothermal (no temperature derating — that is a Day
// 13 refinement), the Sun is a point source at 1 AU (no seasonal
// ±3.4% variation of S), and incidence is a single cosine for the whole
// array (no per-face illumination — the Day 6 simulator treats the array
// as one equivalent flat panel facing the Sun at angle θ). The eclipse
// itself is never double-counted: SunFactor comes from the orbit engine
// (Day 3) and this package only multiplies by it.
package power

import (
	"fmt"
	"math"

	"github.com/LlvallVeiz/powerorbit/internal/config"
	"github.com/LlvallVeiz/powerorbit/internal/orbit"
)

// Panel is a photovoltaic array: one equivalent flat panel described by
// its area, cell efficiency, MPPT/path efficiency, and yearly
// degradation. Build it with NewPanel so the values are validated.
type Panel struct {
	// AreaM2 is the total illuminated panel area, in m².
	AreaM2 float64
	// Efficiency is the cell conversion efficiency, 0..1.
	Efficiency float64
	// MPPTefficiency is the maximum-power-point tracker / EPS path
	// efficiency, 0..1.
	MPPTefficiency float64
	// DegradationPerYear is the fractional power loss per year of mission
	// life, 0..0.5 (e.g. 0.02 = 2%/year).
	DegradationPerYear float64
}

// NewPanel builds a Panel from a mission config's solar_panel section
// and validates it. It returns an error describing every problem found.
func NewPanel(cfg config.SolarPanel) (Panel, error) {
	p := Panel{
		AreaM2:             cfg.AreaM2,
		Efficiency:         cfg.Efficiency,
		MPPTefficiency:     cfg.MPPTefficiency,
		DegradationPerYear: cfg.DegradationPerYear,
	}
	if err := p.Validate(); err != nil {
		return Panel{}, err
	}
	return p, nil
}

// Validate checks the panel parameters against the same engineering
// ranges the mission config enforces (config package): a panel must
// have positive area, efficiencies in (0, 1], and degradation in
// [0, 0.5]/year.
func (p Panel) Validate() error {
	if p.AreaM2 <= 0 || p.AreaM2 > 10 {
		return fmt.Errorf("panel area_m2 must be in (0, 10] m², got %g", p.AreaM2)
	}
	if p.Efficiency <= 0 || p.Efficiency > 1 {
		return fmt.Errorf("panel efficiency must be in (0, 1], got %g", p.Efficiency)
	}
	if p.MPPTefficiency <= 0 || p.MPPTefficiency > 1 {
		return fmt.Errorf("panel mppt_efficiency must be in (0, 1], got %g", p.MPPTefficiency)
	}
	if p.DegradationPerYear < 0 || p.DegradationPerYear > 0.5 {
		return fmt.Errorf("panel degradation_per_year must be in [0, 0.5], got %g", p.DegradationPerYear)
	}
	return nil
}

// DegradationFactor returns the fraction of beginning-of-life (BOL)
// power remaining after the given mission age: (1 − d)^years. The
// compounding form is used because each year's loss applies to the
// already-degraded array, not to the BOL rating.
func (p Panel) DegradationFactor(years float64) (float64, error) {
	if years < 0 {
		return 0, fmt.Errorf("mission age in years must be >= 0, got %g", years)
	}
	return math.Pow(1-p.DegradationPerYear, years), nil
}

// RawPowerW returns the power the array produces before the MPPT/EPS
// path losses, in watts: S · A · η · cos θ · SunFactor · (1 − d)^years.
//
// cosTheta is the cosine of the Sun incidence angle on the panel. A
// negative cosine means the panel faces away from the Sun and produces
// nothing, so it is clamped to 0; a value outside [-1, 1] is not a
// cosine at all and is rejected. sunFactor is the illumination from the
// orbit engine (0 in eclipse, 1 in sunlight) and must be in [0, 1].
// years is the mission age in years (>= 0) for the degradation factor.
func (p Panel) RawPowerW(cosTheta, sunFactor, years float64) (float64, error) {
	if cosTheta < -1 || cosTheta > 1 {
		return 0, fmt.Errorf("cos(theta) must be in [-1, 1], got %g", cosTheta)
	}
	if sunFactor < 0 || sunFactor > 1 {
		return 0, fmt.Errorf("sun factor must be in [0, 1], got %g", sunFactor)
	}
	degradation, err := p.DegradationFactor(years)
	if err != nil {
		return 0, err
	}
	if cosTheta < 0 {
		cosTheta = 0
	}
	return orbit.SolarConstantWPerM2 * p.AreaM2 * p.Efficiency * cosTheta * sunFactor * degradation, nil
}

// PowerW returns the power delivered to the bus after the MPPT/EPS path
// losses, in watts: RawPowerW × η_mppt. This is the number the Day 6
// simulator adds to the battery; RawPowerW is the array rating.
func (p Panel) PowerW(cosTheta, sunFactor, years float64) (float64, error) {
	raw, err := p.RawPowerW(cosTheta, sunFactor, years)
	if err != nil {
		return 0, err
	}
	return raw * p.MPPTefficiency, nil
}

// PeakPowerW returns the beginning-of-life power delivered at normal
// incidence (cos θ = 1) in full sunlight — the array's headline rating.
// For the 1U reference config (0.01 m², 30% GaAs, 90% MPPT) that is
// 1361 × 0.01 × 0.30 × 0.90 = 3.675 W.
func (p Panel) PeakPowerW() float64 {
	return orbit.SolarConstantWPerM2 * p.AreaM2 * p.Efficiency * p.MPPTefficiency
}

// OrbitAveragePowerW returns the average power delivered over one orbit
// at the given incidence angle and mission age, in watts: the peak power
// scaled by the orbit's sunlit fraction (from the orbit engine) and the
// degradation factor. The incidence angle is held constant over the
// orbit — the same single-panel approximation the rest of the v1 model
// uses; a per-face model is a later refinement.
func (p Panel) OrbitAveragePowerW(o orbit.CircularOrbit, cosTheta, years float64) (float64, error) {
	if cosTheta < -1 || cosTheta > 1 {
		return 0, fmt.Errorf("cos(theta) must be in [-1, 1], got %g", cosTheta)
	}
	degradation, err := p.DegradationFactor(years)
	if err != nil {
		return 0, err
	}
	if cosTheta < 0 {
		cosTheta = 0
	}
	return orbit.SolarConstantWPerM2 * p.AreaM2 * p.Efficiency * cosTheta * o.SunlitFraction() * degradation * p.MPPTefficiency, nil
}
