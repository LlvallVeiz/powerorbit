package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validYAML is a minimal valid mission used as the base for mutation tests.
const validYAML = `
name: test-mission
orbit:
  altitude_km: 500
  beta_deg: 0
solar_panel:
  area_m2: 0.01
  efficiency: 0.30
  mppt_efficiency: 0.90
  degradation_per_year: 0.02
battery:
  capacity_wh: 9.62
  charge_efficiency: 0.95
  discharge_efficiency: 0.95
  min_soc: 0.20
  max_dod: 0.30
  initial_soc: 1.0
loads:
  - name: bus
    power_w: 0.5
    duty_cycle: 1.0
simulation:
  orbits: 16
  step_minutes: 1
`

func TestLoadBytesValid(t *testing.T) {
	m, err := LoadBytes([]byte(validYAML))
	if err != nil {
		t.Fatalf("LoadBytes(valid) returned error: %v", err)
	}
	if m.Name != "test-mission" {
		t.Errorf("Name = %q, want %q", m.Name, "test-mission")
	}
	if m.Orbit.AltitudeKm != 500 || m.Orbit.BetaDeg != 0 {
		t.Errorf("Orbit = %+v, want altitude 500 km, beta 0°", m.Orbit)
	}
	if m.SolarPanel.AreaM2 != 0.01 || m.SolarPanel.Efficiency != 0.30 {
		t.Errorf("SolarPanel = %+v, want area 0.01 m², efficiency 0.30", m.SolarPanel)
	}
	if m.Battery.CapacityWh != 9.62 {
		t.Errorf("Battery.CapacityWh = %g, want 9.62", m.Battery.CapacityWh)
	}
	if len(m.Loads) != 1 || m.Loads[0].Name != "bus" {
		t.Errorf("Loads = %+v, want one mode named bus", m.Loads)
	}
	if m.Simulation.Orbits != 16 || m.Simulation.StepMinutes != 1 {
		t.Errorf("Simulation = %+v, want 16 orbits, 1 min step", m.Simulation)
	}
}

func TestLoadReferenceConfig(t *testing.T) {
	// The shipped 1U reference config must always load and validate:
	// it is the canonical example users copy.
	path := filepath.Join("..", "..", "configs", "1u-reference.yaml")
	m, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%q) returned error: %v", path, err)
	}
	if m.Name != "1u-reference" {
		t.Errorf("Name = %q, want %q", m.Name, "1u-reference")
	}
	// Grounded in the literature (plan §2): 500 km, beta 0°, 1361 W/m²
	// solar constant, GaAs > 30%, DoD <= 30%.
	if m.Orbit.AltitudeKm != 500 {
		t.Errorf("reference altitude = %g km, want 500", m.Orbit.AltitudeKm)
	}
	if m.Battery.MaxDoD != 0.30 || m.Battery.MinSoC != 0.20 {
		t.Errorf("reference DoD/SoC rules = %g/%g, want 0.30/0.20",
			m.Battery.MaxDoD, m.Battery.MinSoC)
	}
	// Orbit-average load: 0.5*1.0 + 1.2*0.10 + 3.0*0.10 + 4.0*0.05 = 1.12 W.
	if got := m.AverageLoadW(); got != 1.12 {
		t.Errorf("AverageLoadW() = %g W, want 1.12 W", got)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err == nil {
		t.Fatal("Load(missing) returned nil error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Load(missing) error = %v, want it to wrap ErrNotFound", err)
	}
}

func TestLoadBytesRejectsUnknownFields(t *testing.T) {
	// A typo in a field name must fail loudly, not silently become a default.
	bad := strings.Replace(validYAML, "altitude_km: 500", "altitude_kmm: 500", 1)
	if _, err := LoadBytes([]byte(bad)); err == nil {
		t.Error("LoadBytes with unknown field 'altitude_kmm' returned nil error")
	}
}

func TestLoadBytesRejectsMalformedYAML(t *testing.T) {
	if _, err := LoadBytes([]byte("name: [unclosed")); err == nil {
		t.Error("LoadBytes(malformed YAML) returned nil error")
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Mission)
		want string // substring expected in the error
	}{
		{"empty name", func(m *Mission) { m.Name = "" }, "name must not be empty"},
		{"altitude below LEO", func(m *Mission) { m.Orbit.AltitudeKm = 150 }, "altitude_km"},
		{"altitude above LEO", func(m *Mission) { m.Orbit.AltitudeKm = 36000 }, "altitude_km"},
		{"beta out of range", func(m *Mission) { m.Orbit.BetaDeg = 120 }, "beta_deg"},
		{"zero panel area", func(m *Mission) { m.SolarPanel.AreaM2 = 0 }, "area_m2"},
		{"negative panel area", func(m *Mission) { m.SolarPanel.AreaM2 = -0.01 }, "area_m2"},
		{"efficiency above 1", func(m *Mission) { m.SolarPanel.Efficiency = 1.2 }, "efficiency"},
		{"zero efficiency", func(m *Mission) { m.SolarPanel.Efficiency = 0 }, "efficiency"},
		{"mppt efficiency above 1", func(m *Mission) { m.SolarPanel.MPPTefficiency = 1.5 }, "mppt_efficiency"},
		{"degradation above 50%/yr", func(m *Mission) { m.SolarPanel.DegradationPerYear = 0.6 }, "degradation_per_year"},
		{"zero battery capacity", func(m *Mission) { m.Battery.CapacityWh = 0 }, "capacity_wh"},
		{"charge efficiency above 1", func(m *Mission) { m.Battery.ChargeEfficiency = 1.1 }, "charge_efficiency"},
		{"min SoC negative", func(m *Mission) { m.Battery.MinSoC = -0.1 }, "min_soc"},
		{"max DoD above 1", func(m *Mission) { m.Battery.MaxDoD = 1.2 }, "max_dod"},
		{"initial SoC at min SoC", func(m *Mission) { m.Battery.InitialSoC = 0.20 }, "initial_soc"},
		{"DoD + min SoC contradict", func(m *Mission) { m.Battery.MaxDoD = 0.9 }, "max_dod"},
		{"no loads", func(m *Mission) { m.Loads = nil }, "at least one mode"},
		{"duplicate load name", func(m *Mission) {
			m.Loads = append(m.Loads, LoadMode{Name: "bus", PowerW: 1, DutyCycle: 0.5})
		}, "duplicated"},
		{"negative load power", func(m *Mission) { m.Loads[0].PowerW = -1 }, "power_w"},
		{"duty cycle above 1", func(m *Mission) { m.Loads[0].DutyCycle = 1.5 }, "duty_cycle"},
		{"zero orbits", func(m *Mission) { m.Simulation.Orbits = 0 }, "orbits"},
		{"zero step", func(m *Mission) { m.Simulation.StepMinutes = 0 }, "step_minutes"},
		{"step above 60 min", func(m *Mission) { m.Simulation.StepMinutes = 120 }, "step_minutes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := LoadBytes([]byte(validYAML))
			if err != nil {
				t.Fatalf("setup LoadBytes failed: %v", err)
			}
			tc.mut(m)
			err = m.Validate()
			if err == nil {
				t.Fatalf("Validate() returned nil error, want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate() error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestValidateAggregatesProblems(t *testing.T) {
	// One bad file should report every problem at once, not just the first.
	m, err := LoadBytes([]byte(validYAML))
	if err != nil {
		t.Fatalf("setup LoadBytes failed: %v", err)
	}
	m.Orbit.AltitudeKm = 100
	m.SolarPanel.Efficiency = 2
	m.Simulation.Orbits = 0
	err = m.Validate()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Validate() error type = %T, want *ValidationError", err)
	}
	if len(ve.Problems) < 3 {
		t.Errorf("got %d problems, want >= 3: %v", len(ve.Problems), ve.Problems)
	}
}

func TestAverageLoadW(t *testing.T) {
	m, err := LoadBytes([]byte(validYAML))
	if err != nil {
		t.Fatalf("setup LoadBytes failed: %v", err)
	}
	// bus only: 0.5 W * 1.0 duty = 0.5 W average.
	if got := m.AverageLoadW(); got != 0.5 {
		t.Errorf("AverageLoadW() = %g, want 0.5", got)
	}
}

func TestLoadRoundTripThroughFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mission.yaml")
	if err := os.WriteFile(path, []byte(validYAML), 0o644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	m, err := Load(path)
	if err != nil {
		t.Fatalf("Load(temp file) returned error: %v", err)
	}
	if !m.IsValid() {
		t.Error("IsValid() = false for a config that just loaded")
	}
}
