package integration_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
)

func TestEstimatedStrictness_MaxWithDefault(t *testing.T) {
	// given: estimated alert with default fog
	cfg := domain.StrictnessConfig{
		Default: domain.StrictnessFog,
	}
	estimated := map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessAlert}

	// when
	got := harness.ResolveStrictness(cfg, estimated, []string{"auth-module"})

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
	}
	estimated := map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessAlert}

	// when
	got := harness.ResolveStrictness(cfg, estimated, []string{"auth-module"})

	// then
	if got != domain.StrictnessLockdown {
		t.Errorf("expected lockdown, got %s", got)
	}
}

func TestEstimatedStrictness_DefaultStrongerThanEstimated(t *testing.T) {
	// given: default alert is stronger than estimated fog
	cfg := domain.StrictnessConfig{
		Default: domain.StrictnessAlert,
	}
	estimated := map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessFog}

	// when
	got := harness.ResolveStrictness(cfg, estimated, []string{"auth-module"})

	// then: default alert wins
	if got != domain.StrictnessAlert {
		t.Errorf("expected alert (default stronger), got %s", got)
	}
}

func TestEstimatedStrictness_ClusterKeyUsedForLookup(t *testing.T) {
	// given: cluster key "auth-module" matches estimated
	cfg := domain.StrictnessConfig{
		Default: domain.StrictnessFog,
	}
	estimated := map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessLockdown}
	// Simulate StrictnessKeys output: [clusterName, clusterKey, ...labels]
	keys := []string{"Auth Module", "auth-module", "security"}

	// when
	got := harness.ResolveStrictness(cfg, estimated, keys)

	// then: matched via "auth-module" key
	if got != domain.StrictnessLockdown {
		t.Errorf("expected lockdown via cluster key, got %s", got)
	}
}

func TestEstimatedStrictness_NoMatchFallsToDefault(t *testing.T) {
	// given: estimated for different cluster
	cfg := domain.StrictnessConfig{
		Default: domain.StrictnessFog,
	}
	estimated := map[string]domain.StrictnessLevel{"payment": domain.StrictnessLockdown}

	// when
	got := harness.ResolveStrictness(cfg, estimated, []string{"auth-module"})

	// then: no match, falls to default
	if got != domain.StrictnessFog {
		t.Errorf("expected fog default, got %s", got)
	}
}
