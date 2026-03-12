// white-box-reason: tests unexported checkClaudeAuth, checkLinearMCP, and checkClaudeInference pure functions
package session

import (
	"errors"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestCheckClaudeAuth_Success(t *testing.T) {
	// given: mcp list succeeded
	result := checkClaudeAuth("linear  ✓  connected\n", nil)

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected OK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckClaudeAuth_Error(t *testing.T) {
	// given: mcp list failed
	result := checkClaudeAuth("", errors.New("exit status 1"))

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %v: %s", result.Status, result.Message)
	}
	if result.Name != "claude-auth" {
		t.Errorf("expected name 'claude-auth', got %q", result.Name)
	}
}

func TestCheckLinearMCP_Connected(t *testing.T) {
	// given: mcp list output contains linear connected line
	output := "  linear        ✓  connected  \n  some-other    ✓  connected\n"
	result := checkLinearMCP(output, nil)

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected OK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckLinearMCP_NotConnected(t *testing.T) {
	// given: mcp list output without linear
	output := "  some-other    ✓  connected\n"
	result := checkLinearMCP(output, nil)

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckLinearMCP_McpError(t *testing.T) {
	// given: mcp list command failed
	result := checkLinearMCP("", errors.New("exit status 1"))

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckLinearMCP_Disconnected(t *testing.T) {
	// given: linear exists but is disconnected (no ✓)
	output := "  linear        ✗  disconnected\n"
	result := checkLinearMCP(output, nil)

	// then: should warn because ✓ is required
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN for disconnected, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckClaudeInference_Success(t *testing.T) {
	// given
	output := "2"

	// when
	result := checkClaudeInference(output, nil)

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected OK, got %v: %s", result.Status, result.Message)
	}
	if result.Message != "inference OK" {
		t.Errorf("expected 'inference OK', got: %s", result.Message)
	}
}

func TestCheckClaudeInference_Error(t *testing.T) {
	// given
	result := checkClaudeInference("", errors.New("exit status 1"))

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckClaudeInference_FalsePositive(t *testing.T) {
	// given: output contains "2" as substring but is not exactly "2"
	result := checkClaudeInference("12", nil)

	// then: must warn — "12" is not the correct answer
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN for false positive '12', got %v: %s", result.Status, result.Message)
	}
}

func TestCheckClaudeInference_UnexpectedResponse(t *testing.T) {
	// given
	result := checkClaudeInference("I cannot compute that", nil)

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %v: %s", result.Status, result.Message)
	}
	if !strings.HasPrefix(result.Message, "unexpected response: ") {
		t.Errorf("expected message starting with 'unexpected response: ', got: %s", result.Message)
	}
}
