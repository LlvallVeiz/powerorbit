// Package orbit is the v1 orbit engine for powerorbit: a circular LEO orbit
// plus a cylindrical Earth-shadow eclipse model.
//
// This is a first-order approximation of the kind student CubeSat teams
// standardly use for power budgeting — good enough to size a panel and a
// battery, explicitly not a flight-dynamics propagator. Full SGP4 from a
// real TLE lands on Day 11; until then the geometry below is validated
// against the published numbers in the project plan (rencana-powerorbit.md
// §2), so every constant and every result has a literature anchor:
//
//   - 500 km LEO: period 94 min 37 s, eclipse 35 min 45 s at beta = 0°
//     (1U EPS design paper; BIRDS: 36 min at beta = 0°)
//   - 600 km SSO: eclipse max 35.45 min of a 96.66 min orbit (IOP study)
//   - Eclipse vanishes at high beta (BIRDS: 0 min at beta = 73°)
//
// Geometry (cylindrical shadow). Let a = R + h be the orbit radius and
// beta the angle between the Sun direction and the orbit plane. The
// satellite is inside the shadow cylinder when its distance from the
// Sun–Earth line is below R and it is on the anti-Sun side. Measuring the
// orbit angle theta from the sub-solar point (theta = 0 closest to the
// Sun, theta = 180° at the anti-Sun point), the eclipse is centred on
// theta = 180° with half-angle
//
//	phi_max = arccos( sqrt(1 - (R/a)^2) / cos(beta) )
//
// which exists only while cos(beta) >= sqrt(1 - (R/a)^2), i.e. while
// |beta| <= beta_max = arcsin(R/a). The eclipse fraction of one orbit is
// then 2*phi_max / 360°.
package orbit

import (
	"errors"
	"fmt"
	"math"
)

// MuEarthKm3PerS2 is Earth's gravitational parameter, km³/s² (WGS-84 value
// used by SGP4).
const MuEarthKm3PerS2 = 398600.4418

// EarthRadiusKm is Earth's radius, in km. The WGS-84 equatorial radius
// (6378.137 km) is used rather than the mean radius (6371 km) because it
// is the value SGP4 itself uses — and because it is the radius that
// reproduces the published reference numbers: a 500 km orbit then has a
// 94 min 37 s period and a 35 min 45 s eclipse at beta = 0°, and a 600 km
// orbit a 96.66 min period with a 35.45 min eclipse (IOP study). With the
// mean radius the same formulas give 94 min 28 s, 9 s short of the
// literature value.
const EarthRadiusKm = 6378.137

// SolarConstantWPerM2 is the solar irradiance at 1 AU, in W/m².
const SolarConstantWPerM2 = 1361.0

// MinAltitudeKm and MaxAltitudeKm bound the LEO range this engine models —
// the same 200–2000 km range the mission config enforces (config package),
// because SGP4 (Day 11) is only valid for near-Earth orbits.
const (
	MinAltitudeKm = 200.0
	MaxAltitudeKm = 2000.0
)

// CircularOrbit is a circular low Earth orbit described by altitude and
// beta angle — everything the v1 power simulation needs to know about
// where the satellite is relative to the Sun.
type CircularOrbit struct {
	// AltitudeKm is the orbit altitude above the Earth's surface, in km.
	AltitudeKm float64
	// BetaDeg is the beta angle (angle between the orbit plane and the
	// Sun direction), in degrees, -90..90. Beta = 0° gives the longest
	// eclipse; |beta| >= beta_max gives no eclipse at all.
	BetaDeg float64
}

// Validate checks the orbit parameters against the LEO range.
func (o CircularOrbit) Validate() error {
	if o.AltitudeKm < MinAltitudeKm || o.AltitudeKm > MaxAltitudeKm {
		return fmt.Errorf("altitude_km must be in [%g, %g] km (LEO), got %g",
			MinAltitudeKm, MaxAltitudeKm, o.AltitudeKm)
	}
	if o.BetaDeg < -90 || o.BetaDeg > 90 {
		return fmt.Errorf("beta_deg must be in [-90, 90], got %g", o.BetaDeg)
	}
	return nil
}

// SemiMajorAxisKm returns the orbit radius a = R + h, in km.
func (o CircularOrbit) SemiMajorAxisKm() float64 {
	return EarthRadiusKm + o.AltitudeKm
}

// PeriodMinutes returns the orbital period from Kepler's third law,
// T = 2*pi*sqrt(a^3/mu).
func (o CircularOrbit) PeriodMinutes() float64 {
	a := o.SemiMajorAxisKm()
	periodS := 2 * math.Pi * math.Sqrt(math.Pow(a, 3)/MuEarthKm3PerS2)
	return periodS / 60
}

// BetaMaxDeg returns the largest |beta| that still produces an eclipse:
// beta_max = arcsin(R/a). Above it the orbit plane is tilted so far toward
// the Sun that the satellite never enters the shadow cylinder.
func (o CircularOrbit) BetaMaxDeg() float64 {
	return math.Asin(EarthRadiusKm/o.SemiMajorAxisKm()) * 180 / math.Pi
}

// EclipseHalfAngleDeg returns the angular half-width of the eclipse
// measured from the anti-Sun point (theta = 180°), in degrees, and
// whether an eclipse exists at all at this beta angle.
func (o CircularOrbit) EclipseHalfAngleDeg() (float64, bool) {
	// sin(phi) geometry: cos(phi_max) = sqrt(1 - (R/a)^2) / cos(beta).
	// No eclipse when cos(beta) is too small for the ratio to stay <= 1.
	a := o.SemiMajorAxisKm()
	cosBeta := math.Cos(o.BetaDeg * math.Pi / 180)
	ratio := math.Sqrt(1-math.Pow(EarthRadiusKm/a, 2)) / cosBeta
	if ratio > 1 {
		return 0, false
	}
	return math.Acos(ratio) * 180 / math.Pi, true
}

// EclipseFraction returns the fraction of one orbit spent in eclipse,
// in [0, 1]. It is 0 when |beta| >= beta_max.
func (o CircularOrbit) EclipseFraction() float64 {
	phiMax, ok := o.EclipseHalfAngleDeg()
	if !ok {
		return 0
	}
	return 2 * phiMax / 360
}

// EclipseMinutes returns the eclipse duration per orbit, in minutes.
func (o CircularOrbit) EclipseMinutes() float64 {
	return o.EclipseFraction() * o.PeriodMinutes()
}

// SunlightMinutes returns the sunlit duration per orbit, in minutes.
func (o CircularOrbit) SunlightMinutes() float64 {
	return o.PeriodMinutes() - o.EclipseMinutes()
}

// SunlitFraction returns the fraction of one orbit spent in sunlight.
func (o CircularOrbit) SunlitFraction() float64 {
	return 1 - o.EclipseFraction()
}

// IsEclipsed reports whether the satellite is in Earth's shadow at the
// given orbit angle theta, in degrees measured from the sub-solar point
// (theta = 0 closest to the Sun, theta = 180° at the anti-Sun point).
// The eclipse is centred on theta = 180° with half-angle phi_max.
func (o CircularOrbit) IsEclipsed(orbitAngleDeg float64) bool {
	phiMax, ok := o.EclipseHalfAngleDeg()
	if !ok {
		return false
	}
	// Angular distance from the anti-Sun point, wrapped to [0, 180].
	d := math.Mod(math.Abs(orbitAngleDeg-180), 360)
	if d > 180 {
		d = 360 - d
	}
	return d <= phiMax
}

// SunFactor returns 1.0 in sunlight and 0.0 in eclipse — the illumination
// multiplier the PV model (Day 4) applies to panel power. The v1 model is
// a hard on/off switch (umbra only); penumbra is a Day 11+ refinement.
func (o CircularOrbit) SunFactor(orbitAngleDeg float64) float64 {
	if o.IsEclipsed(orbitAngleDeg) {
		return 0
	}
	return 1
}

// ErrInvalidOrbit is returned by constructors that reject an orbit.
var ErrInvalidOrbit = errors.New("invalid orbit")

// New validates the parameters and returns the orbit, or an error.
func New(altitudeKm, betaDeg float64) (CircularOrbit, error) {
	o := CircularOrbit{AltitudeKm: altitudeKm, BetaDeg: betaDeg}
	if err := o.Validate(); err != nil {
		return CircularOrbit{}, fmt.Errorf("%w: %w", ErrInvalidOrbit, err)
	}
	return o, nil
}
