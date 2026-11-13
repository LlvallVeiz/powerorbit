package orbit

import (
	"math"
	"testing"
)

// approx reports whether got is within tol of want.
func approx(t *testing.T, what string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.4f, want %.4f ± %.4f", what, got, want, tol)
	}
}

func TestPeriod500KmMatchesLiterature(t *testing.T) {
	// 1U EPS design paper: 94 min 37 s at 500 km.
	o := CircularOrbit{AltitudeKm: 500, BetaDeg: 0}
	approx(t, "period at 500 km (min)", o.PeriodMinutes(), 94.62, 0.1)
}

func TestPeriod600KmMatchesLiterature(t *testing.T) {
	// IOP energy-balance study of a 3U CubeSat: 96.66 min orbit at 600 km.
	o := CircularOrbit{AltitudeKm: 600, BetaDeg: 0}
	approx(t, "period at 600 km (min)", o.PeriodMinutes(), 96.66, 0.1)
}

func TestPeriodGrowsWithAltitude(t *testing.T) {
	low := CircularOrbit{AltitudeKm: 300, BetaDeg: 0}.PeriodMinutes()
	high := CircularOrbit{AltitudeKm: 800, BetaDeg: 0}.PeriodMinutes()
	if high <= low {
		t.Errorf("higher orbit must have a longer period: 300 km = %.2f, 800 km = %.2f", low, high)
	}
}

func TestEclipse500KmBeta0MatchesLiterature(t *testing.T) {
	// 1U EPS design paper: 35 min 45 s eclipse at 500 km, beta = 0°.
	// BIRDS reports 36 min at beta = 0° for the same geometry.
	o := CircularOrbit{AltitudeKm: 500, BetaDeg: 0}
	approx(t, "eclipse at 500 km, beta 0 (min)", o.EclipseMinutes(), 35.75, 0.5)
}

func TestEclipse600KmBeta0MatchesLiterature(t *testing.T) {
	// IOP study: eclipse max 35.45 min of a 96.66 min orbit at 600 km SSO.
	o := CircularOrbit{AltitudeKm: 600, BetaDeg: 0}
	approx(t, "eclipse at 600 km, beta 0 (min)", o.EclipseMinutes(), 35.45, 0.5)
}

func TestEclipse400KmBeta30NearBIRDS(t *testing.T) {
	// BIRDS analysis: ~35 min eclipse at beta = 30°. The cylindrical
	// shadow model gives 34.4 min at 400 km — the gap is the penumbra
	// and the exact orbit the BIRDS number was computed for, so the
	// test allows a 1 min tolerance around the model value and checks
	// the direction (shorter than the beta = 0 eclipse).
	o := CircularOrbit{AltitudeKm: 400, BetaDeg: 30}
	approx(t, "eclipse at 400 km, beta 30 (min)", o.EclipseMinutes(), 34.4, 1.0)
	worst := CircularOrbit{AltitudeKm: 400, BetaDeg: 0}.EclipseMinutes()
	if got := o.EclipseMinutes(); got >= worst {
		t.Errorf("eclipse must shrink as |beta| grows: beta 30 = %.2f, beta 0 = %.2f", got, worst)
	}
}

func TestEclipseVanishesAtHighBeta(t *testing.T) {
	// BIRDS: 0 min eclipse at beta = 73°. The model's geometric limit
	// at 500 km is beta_max = arcsin(R/a) ≈ 68°, so 73° is safely past it.
	o := CircularOrbit{AltitudeKm: 500, BetaDeg: 73}
	if got := o.EclipseMinutes(); got != 0 {
		t.Errorf("eclipse at beta 73° = %.4f min, want 0", got)
	}
	if got := o.EclipseFraction(); got != 0 {
		t.Errorf("eclipse fraction at beta 73° = %.4f, want 0", got)
	}
	if o.IsEclipsed(180) {
		t.Error("no angle may be eclipsed when beta is past beta_max")
	}
}

func TestBetaMax500Km(t *testing.T) {
	// arcsin(6371/6871) ≈ 68.0°.
	o := CircularOrbit{AltitudeKm: 500, BetaDeg: 0}
	approx(t, "beta_max at 500 km (deg)", o.BetaMaxDeg(), 68.0, 0.5)
}

func TestEclipseShrinksMonotonicallyWithBeta(t *testing.T) {
	prev := math.Inf(1)
	for _, beta := range []float64{0, 15, 30, 45, 60, 67} {
		got := CircularOrbit{AltitudeKm: 500, BetaDeg: beta}.EclipseMinutes()
		if got >= prev {
			t.Errorf("eclipse at beta %.0f° = %.2f min, not smaller than previous %.2f", beta, got, prev)
		}
		prev = got
	}
}

func TestSunlightPlusEclipseEqualsPeriod(t *testing.T) {
	for _, tc := range []CircularOrbit{
		{AltitudeKm: 400, BetaDeg: 0},
		{AltitudeKm: 500, BetaDeg: 30},
		{AltitudeKm: 800, BetaDeg: 80},
	} {
		sum := tc.SunlightMinutes() + tc.EclipseMinutes()
		approx(t, "sunlight + eclipse (min)", sum, tc.PeriodMinutes(), 1e-9)
		approx(t, "sunlit + eclipse fraction", tc.SunlitFraction()+tc.EclipseFraction(), 1, 1e-12)
	}
}

func TestIsEclipsedGeometry(t *testing.T) {
	// At 500 km, beta = 0°, the eclipse spans theta = 180° ± ~68°,
	// i.e. roughly 112°..248° from the sub-solar point.
	o := CircularOrbit{AltitudeKm: 500, BetaDeg: 0}
	eclipsed := []float64{180, 130, 230, 112.5, 247.5}
	for _, a := range eclipsed {
		if !o.IsEclipsed(a) {
			t.Errorf("angle %.1f° should be eclipsed", a)
		}
	}
	sunlit := []float64{0, 90, 270, 359, 100, 260}
	for _, a := range sunlit {
		if o.IsEclipsed(a) {
			t.Errorf("angle %.1f° should be sunlit", a)
		}
	}
}

func TestSunFactorIsOnOff(t *testing.T) {
	o := CircularOrbit{AltitudeKm: 500, BetaDeg: 0}
	if got := o.SunFactor(180); got != 0 {
		t.Errorf("SunFactor(180°) = %v, want 0 (eclipse)", got)
	}
	if got := o.SunFactor(0); got != 1 {
		t.Errorf("SunFactor(0°) = %v, want 1 (sunlight)", got)
	}
}

func TestNewRejectsInvalidOrbits(t *testing.T) {
	for _, tc := range []struct {
		alt, beta float64
	}{
		{100, 0},    // below LEO
		{3000, 0},   // above LEO
		{500, 100},  // beta out of range
		{500, -100}, // beta out of range
	} {
		if _, err := New(tc.alt, tc.beta); err == nil {
			t.Errorf("New(%v, %v) should fail", tc.alt, tc.beta)
		}
	}
	if _, err := New(500, 0); err != nil {
		t.Errorf("New(500, 0) should succeed, got %v", err)
	}
}

func TestReferenceMissionOrbit(t *testing.T) {
	// The shipped 1U reference config flies 500 km at beta = 0°: its
	// orbit is a 94.6 min period with a ~35.7 min eclipse — the numbers
	// the Day 6 simulator will integrate over.
	o := CircularOrbit{AltitudeKm: 500, BetaDeg: 0}
	approx(t, "1U reference period (min)", o.PeriodMinutes(), 94.62, 0.1)
	approx(t, "1U reference eclipse (min)", o.EclipseMinutes(), 35.75, 0.5)
}
