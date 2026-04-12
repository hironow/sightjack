// white-box-reason: tests unexported assembly helpers for ProviderAdapterConfig field omission guard
package session

import (
	"reflect"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestAdapterConfigFromDomainConfig_AllFieldsPopulated(t *testing.T) {
	cfg := &domain.Config{
		ClaudeCmd:  "test-claude",
		Model:      "test-model",
		TimeoutSec: 42,
	}
	pac := AdapterConfigFromDomainConfig(cfg, "/test/base")

	v := reflect.ValueOf(pac)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).IsZero() {
			t.Errorf("field %s is zero — helper omitted it", v.Type().Field(i).Name)
		}
	}
}

func TestRetryConfigFromDomainConfig_AllFieldsPopulated(t *testing.T) {
	cfg := &domain.Config{
		TimeoutSec: 60,
		Retry: domain.RetryConfig{
			MaxAttempts:  3,
			BaseDelaySec: 5,
		},
	}
	rc := RetryConfigFromDomainConfig(cfg)

	v := reflect.ValueOf(rc)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).IsZero() {
			t.Errorf("field %s is zero — helper omitted it", v.Type().Field(i).Name)
		}
	}
}
