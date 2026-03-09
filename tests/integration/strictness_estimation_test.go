package integration_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestEstimatedStrictness_MaxWithDefault(t *testing.T) {
	// given: estimated alert with default fog
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessFog,
		Estimated: map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessAlert},
	}

	// when
	got := domain.ResolveStrictness(cfg, []string{"auth-module"})

	// then: estimated alert wins over default fog
	if got != domain.StrictnessAlert {
		t.Errorf("expected alert, got %s", got)
	}
}

func TestEstimatedStrictness_OverrideWins(t *testing.T) {
	// given: override lockdown beats estimated alert
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessFog,
		Overrides: map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessLockdown},
		Estimated: map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessAlert},
	}

	// when
	got := domain.ResolveStrictness(cfg, []string{"auth-module"})

	// then
	if got != domain.StrictnessLockdown {
		t.Errorf("expected lockdown, got %s", got)
	}
}

func TestEstimatedStrictness_DefaultStrongerThanEstimated(t *testing.T) {
	// given: default alert is stronger than estimated fog
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessAlert,
		Estimated: map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessFog},
	}

	// when
	got := domain.ResolveStrictness(cfg, []string{"auth-module"})

	// then: default alert wins
	if got != domain.StrictnessAlert {
		t.Errorf("expected alert (default stronger), got %s", got)
	}
}

func TestEstimatedStrictness_ClusterKeyUsedForLookup(t *testing.T) {
	// given: cluster key "auth-module" matches estimated
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessFog,
		Estimated: map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessLockdown},
	}
	// Simulate StrictnessKeys output: [clusterName, clusterKey, ...labels]
	keys := []string{"Auth Module", "auth-module", "security"}

	// when
	got := domain.ResolveStrictness(cfg, keys)

	// then: matched via "auth-module" key
	if got != domain.StrictnessLockdown {
		t.Errorf("expected lockdown via cluster key, got %s", got)
	}
}

func TestEstimatedStrictness_NoMatchFallsToDefault(t *testing.T) {
	// given: estimated for different cluster
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessFog,
		Estimated: map[string]domain.StrictnessLevel{"payment": domain.StrictnessLockdown},
	}

	// when
	got := domain.ResolveStrictness(cfg, []string{"auth-module"})

	// then: no match, falls to default
	if got != domain.StrictnessFog {
		t.Errorf("expected fog default, got %s", got)
	}
}
