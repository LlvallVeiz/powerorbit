// Package config defines the mission configuration file format for powerorbit
// and validates it.
//
// A mission is described in a single YAML file ("power budget as code"):
// orbit, solar panel, battery, subsystem loads, and simulation settings.
// The file is version-controlled with the repo, and `powerorbit check`
// (Day 12) will enforce it in CI like a unit test.
//
// Reference numbers in configs/1u-reference.yaml come from the published
// literature cited in the project plan (rencana-powerorbit.md §2): a 500 km
// LEO orbit has a ~94.6 min period with ~35 min eclipse at beta = 0°,
// the solar constant is 1361 W/m², triple-junction GaAs cells exceed 30%
// efficiency, and the per-orbit depth-of-discharge rule is <= 30%.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Mission is the top-level mission configuration.
type Mission struct {
	// Name of the mission, e.g. "1u-reference".
	Name string `yaml:"name"`
	// Orbit describes the circular LEO orbit the satellite flies in.
	Orbit Orbit `yaml:"orbit"`
	// SolarPanel describes the photovoltaic array.
	SolarPanel SolarPanel `yaml:"solar_panel"`
	// Battery describes the energy storage (Li-ion 18650 cells).
	Battery Battery `yaml:"battery"`
	// Loads lists one entry per subsystem operating mode.
	Loads []LoadMode `yaml:"loads"`
	// Simulation controls the time-step simulation (Day 6).
	Simulation Simulation `yaml:"simulation"`
}

// Orbit describes a circular low Earth orbit.
type Orbit struct {
	// AltitudeKm is the orbit altitude above the Earth's surface, in km.
	// Valid LEO range: 200–2000 km.
	AltitudeKm float64 `yaml:"altitude_km"`
	// BetaDeg is the beta angle (angle between the orbit plane and the
	// Sun direction), in degrees, -90..90. Beta = 0° gives the longest
	// eclipse (~35 min at 500 km); |beta| >= ~73° gives no eclipse.
	BetaDeg float64 `yaml:"beta_deg"`
}

// SolarPanel describes the photovoltaic array.
type SolarPanel struct {
	// AreaM2 is the total illuminated panel area, in m².
	AreaM2 float64 `yaml:"area_m2"`
	// Efficiency is the cell conversion efficiency, 0..1.
	// Triple-junction GaAs cells exceed 0.30 (see plan §2).
	Efficiency float64 `yaml:"efficiency"`
	// MPPTefficiency is the maximum-power-point tracker / EPS path
	// efficiency, 0..1.
	MPPTefficiency float64 `yaml:"mppt_efficiency"`
	// DegradationPerYear is the fractional power loss per year of mission
	// life, 0..0.5 (e.g. 0.02 = 2%/year).
	DegradationPerYear float64 `yaml:"degradation_per_year"`
}

// Battery describes the Li-ion battery (18650 cells).
type Battery struct {
	// CapacityWh is the nameplate energy capacity, in watt-hours.
	CapacityWh float64 `yaml:"capacity_wh"`
	// ChargeEfficiency is the charge path efficiency, 0..1.
	ChargeEfficiency float64 `yaml:"charge_efficiency"`
	// DischargeEfficiency is the discharge path efficiency, 0..1.
	DischargeEfficiency float64 `yaml:"discharge_efficiency"`
	// MinSoC is the lowest allowed state of charge, 0..1.
	// The engineering rule is SoC_min >= 0.20.
	MinSoC float64 `yaml:"min_soc"`
	// MaxDoD is the largest allowed depth of discharge per orbit, 0..1.
	// The engineering rule is DoD <= 0.30 per orbit (EO-1 reference model).
	MaxDoD float64 `yaml:"max_dod"`
	// InitialSoC is the state of charge at the start of the simulation, 0..1.
	InitialSoC float64 `yaml:"initial_soc"`
}

// LoadMode is one subsystem operating mode with its power draw and duty cycle.
type LoadMode struct {
	// Name of the mode, e.g. "idle", "beacon", "payload", "downlink".
	Name string `yaml:"name"`
	// PowerW is the electrical power drawn while the mode is active, in watts.
	PowerW float64 `yaml:"power_w"`
	// DutyCycle is the fraction of each orbit spent in this mode, 0..1.
	DutyCycle float64 `yaml:"duty_cycle"`
}

// Simulation controls the time-step simulation (Day 6).
type Simulation struct {
	// Orbits is how many consecutive orbits to simulate (>= 1).
	Orbits int `yaml:"orbits"`
	// StepMinutes is the simulation time step, in minutes (> 0, <= 60).
	// The default is 1 minute.
	StepMinutes float64 `yaml:"step_minutes"`
}

// ValidationError aggregates every problem found in a mission file so the
// user can fix them all in one pass instead of one at a time.
type ValidationError struct {
	Problems []string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return "invalid mission config:\n  - " + strings.Join(e.Problems, "\n  - ")
}

// Validate checks a Mission against every engineering and sanity rule and
// returns a *ValidationError listing all problems, or nil if it is valid.
func (m *Mission) Validate() error {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}
	frac := func(field string, v, min, max float64) {
		if v < min || v > max {
			add("%s must be in [%g, %g], got %g", field, min, max, v)
		}
	}

	if strings.TrimSpace(m.Name) == "" {
		add("name must not be empty")
	}

	// Orbit: LEO only — SGP4 (Day 11) is only valid for near-Earth orbits.
	if m.Orbit.AltitudeKm < 200 || m.Orbit.AltitudeKm > 2000 {
		add("orbit.altitude_km must be in [200, 2000] km (LEO), got %g", m.Orbit.AltitudeKm)
	}
	frac("orbit.beta_deg", m.Orbit.BetaDeg, -90, 90)

	// Solar panel.
	if m.SolarPanel.AreaM2 <= 0 || m.SolarPanel.AreaM2 > 10 {
		add("solar_panel.area_m2 must be in (0, 10] m², got %g", m.SolarPanel.AreaM2)
	}
	frac("solar_panel.efficiency", m.SolarPanel.Efficiency, 0, 1)
	if m.SolarPanel.Efficiency <= 0 {
		add("solar_panel.efficiency must be > 0, got %g", m.SolarPanel.Efficiency)
	}
	frac("solar_panel.mppt_efficiency", m.SolarPanel.MPPTefficiency, 0, 1)
	if m.SolarPanel.MPPTefficiency <= 0 {
		add("solar_panel.mppt_efficiency must be > 0, got %g", m.SolarPanel.MPPTefficiency)
	}
	frac("solar_panel.degradation_per_year", m.SolarPanel.DegradationPerYear, 0, 0.5)

	// Battery.
	if m.Battery.CapacityWh <= 0 {
		add("battery.capacity_wh must be > 0, got %g", m.Battery.CapacityWh)
	}
	frac("battery.charge_efficiency", m.Battery.ChargeEfficiency, 0, 1)
	if m.Battery.ChargeEfficiency <= 0 {
		add("battery.charge_efficiency must be > 0, got %g", m.Battery.ChargeEfficiency)
	}
	frac("battery.discharge_efficiency", m.Battery.DischargeEfficiency, 0, 1)
	if m.Battery.DischargeEfficiency <= 0 {
		add("battery.discharge_efficiency must be > 0, got %g", m.Battery.DischargeEfficiency)
	}
	frac("battery.min_soc", m.Battery.MinSoC, 0, 1)
	frac("battery.max_dod", m.Battery.MaxDoD, 0, 1)
	if m.Battery.MaxDoD <= 0 {
		add("battery.max_dod must be > 0, got %g", m.Battery.MaxDoD)
	}
	frac("battery.initial_soc", m.Battery.InitialSoC, 0, 1)
	if m.Battery.InitialSoC <= m.Battery.MinSoC {
		add("battery.initial_soc (%g) must be above battery.min_soc (%g)",
			m.Battery.InitialSoC, m.Battery.MinSoC)
	}
	// DoD and minimum SoC must not contradict each other: the deepest allowed
	// discharge must leave the battery at or above its minimum SoC.
	if m.Battery.MaxDoD+m.Battery.MinSoC > 1 {
		add("battery.max_dod (%g) + battery.min_soc (%g) must be <= 1",
			m.Battery.MaxDoD, m.Battery.MinSoC)
	}

	// Loads.
	if len(m.Loads) == 0 {
		add("loads must list at least one mode")
	}
	seen := make(map[string]bool, len(m.Loads))
	for i, l := range m.Loads {
		prefix := fmt.Sprintf("loads[%d]", i)
		if strings.TrimSpace(l.Name) == "" {
			add("%s.name must not be empty", prefix)
		} else if seen[l.Name] {
			add("%s.name %q is duplicated", prefix, l.Name)
		} else {
			seen[l.Name] = true
		}
		if l.PowerW < 0 {
			add("%s.power_w must be >= 0, got %g", prefix, l.PowerW)
		}
		frac(prefix+".duty_cycle", l.DutyCycle, 0, 1)
	}

	// Simulation.
	if m.Simulation.Orbits < 1 {
		add("simulation.orbits must be >= 1, got %d", m.Simulation.Orbits)
	}
	if m.Simulation.StepMinutes <= 0 || m.Simulation.StepMinutes > 60 {
		add("simulation.step_minutes must be in (0, 60], got %g", m.Simulation.StepMinutes)
	}

	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

// Load reads a mission YAML file from path, parses it strictly (unknown
// fields are rejected — a typo must not silently become a default), and
// validates it. It returns the mission or an error describing the problem.
func Load(path string) (*Mission, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %q", ErrNotFound, path)
		}
		return nil, fmt.Errorf("reading config %q: %w", path, err)
	}
	return LoadBytes(data)
}

// LoadBytes parses mission YAML from memory, strictly and validated.
// It is the same path Load uses, exposed so tests can load configs without
// touching the filesystem.
func LoadBytes(data []byte) (*Mission, error) {
	var m Mission
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true) // reject unknown fields
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parsing config YAML: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// AverageLoadW returns the orbit-average power draw of all load modes,
// weighting each mode by its duty cycle. Used by the simulator (Day 6).
func (m *Mission) AverageLoadW() float64 {
	total := 0.0
	for _, l := range m.Loads {
		total += l.PowerW * l.DutyCycle
	}
	return total
}

// IsValid reports whether the mission passes validation. Prefer Validate
// when the caller needs the list of problems.
func (m *Mission) IsValid() bool { return m.Validate() == nil }

// ErrNotFound is returned by Load when the config file does not exist.
var ErrNotFound = errors.New("config file not found")
