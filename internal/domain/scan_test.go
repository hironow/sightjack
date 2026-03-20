package domain_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestSanitizeName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "lowercase_ascii", in: "auth-module", want: "auth-module"},
		{name: "uppercase_converted", in: "Auth_Module", want: "auth_module"},
		{name: "spaces_to_underscore", in: "my cluster", want: "my_cluster"},
		{name: "special_chars_replaced", in: "foo/bar.baz@qux", want: "foo_bar_baz_qux"},
		{name: "digits_preserved", in: "v2-api-3", want: "v2-api-3"},
		{name: "empty_string", in: "", want: ""},
		{name: "all_special", in: "!@#$%", want: "_____"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// when
			got := domain.SanitizeName(tt.in)

			// then
			if got != tt.want {
				t.Errorf("SanitizeName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDetectFailedClusterNames(t *testing.T) {
	t.Parallel()

	t.Run("all_succeeded", func(t *testing.T) {
		t.Parallel()
		// given
		clusters := []domain.ClusterScanResult{
			{Name: "auth"}, {Name: "billing"},
		}
		successes := []domain.WaveGenerateResult{
			{ClusterName: "auth"}, {ClusterName: "billing"},
		}

		// when
		failed := domain.DetectFailedClusterNames(clusters, successes)

		// then
		if len(failed) != 0 {
			t.Errorf("expected no failures, got %v", failed)
		}
	})

	t.Run("one_failed", func(t *testing.T) {
		t.Parallel()
		// given
		clusters := []domain.ClusterScanResult{
			{Name: "auth"}, {Name: "billing"},
		}
		successes := []domain.WaveGenerateResult{
			{ClusterName: "auth"},
		}

		// when
		failed := domain.DetectFailedClusterNames(clusters, successes)

		// then
		if !failed["billing"] {
			t.Errorf("expected billing to be failed, got %v", failed)
		}
		if failed["auth"] {
			t.Errorf("auth should not be failed")
		}
	})

	t.Run("duplicate_cluster_partial_failure", func(t *testing.T) {
		t.Parallel()
		// given — two instances of "auth", only one succeeded
		clusters := []domain.ClusterScanResult{
			{Name: "auth"}, {Name: "auth"},
		}
		successes := []domain.WaveGenerateResult{
			{ClusterName: "auth"},
		}

		// when
		failed := domain.DetectFailedClusterNames(clusters, successes)

		// then
		if !failed["auth"] {
			t.Errorf("expected auth to be failed (partial), got %v", failed)
		}
	})
}

func TestChunkSlice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		items    []string
		size     int
		wantLen  int
		wantLast []string
	}{
		{name: "empty_input", items: nil, size: 3, wantLen: 0, wantLast: nil},
		{name: "exact_fit", items: []string{"a", "b", "c"}, size: 3, wantLen: 1, wantLast: []string{"a", "b", "c"}},
		{name: "remainder", items: []string{"a", "b", "c", "d", "e"}, size: 2, wantLen: 3, wantLast: []string{"e"}},
		{name: "size_one", items: []string{"a", "b"}, size: 1, wantLen: 2, wantLast: []string{"b"}},
		{name: "zero_size_returns_single_chunk", items: []string{"a", "b"}, size: 0, wantLen: 1, wantLast: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// when
			got := domain.ChunkSlice(tt.items, tt.size)

			// then
			if len(got) != tt.wantLen {
				t.Fatalf("ChunkSlice(%v, %d) produced %d chunks, want %d", tt.items, tt.size, len(got), tt.wantLen)
			}
			if tt.wantLast != nil {
				last := got[len(got)-1]
				if len(last) != len(tt.wantLast) {
					t.Errorf("last chunk = %v, want %v", last, tt.wantLast)
				}
			}
		})
	}
}

func TestClampCompleteness(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{name: "valid", in: 0.5, want: 0.5},
		{name: "negative", in: -0.1, want: 0.0},
		{name: "above_one", in: 1.5, want: 1.0},
		{name: "zero", in: 0.0, want: 0.0},
		{name: "one", in: 1.0, want: 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// when
			got := domain.ClampCompleteness(tt.in)

			// then
			if got != tt.want {
				t.Errorf("ClampCompleteness(%f) = %f, want %f", tt.in, got, tt.want)
			}
		})
	}
}

func TestMergeClusterChunks_ClampsCompleteness(t *testing.T) {
	t.Parallel()
	// given: issues with out-of-bounds completeness
	chunks := []domain.ClusterScanResult{
		{
			Name:   "auth",
			Issues: []domain.IssueDetail{{ID: "1", Completeness: 1.5}},
		},
		{
			Name:   "auth",
			Issues: []domain.IssueDetail{{ID: "2", Completeness: 0.5}},
		},
	}

	// when
	merged := domain.MergeClusterChunks("auth", chunks)

	// then: clamped 1.5->1.0, avg of 1.0 and 0.5 = 0.75
	if merged.Completeness != 0.75 {
		t.Errorf("completeness = %f, want 0.75", merged.Completeness)
	}
}

func TestMergeClusterChunks(t *testing.T) {
	t.Parallel()

	t.Run("merges_issues_and_observations", func(t *testing.T) {
		t.Parallel()
		// given
		chunks := []domain.ClusterScanResult{
			{
				Name:         "auth",
				Issues:       []domain.IssueDetail{{ID: "1", Completeness: 0.8}},
				Observations: []string{"obs1"},
			},
			{
				Name:         "auth",
				Issues:       []domain.IssueDetail{{ID: "2", Completeness: 0.6}},
				Observations: []string{"obs2"},
			},
		}

		// when
		merged := domain.MergeClusterChunks("auth", chunks)

		// then
		if merged.Name != "auth" {
			t.Errorf("name = %q, want %q", merged.Name, "auth")
		}
		if len(merged.Issues) != 2 {
			t.Errorf("issues count = %d, want 2", len(merged.Issues))
		}
		if len(merged.Observations) != 2 {
			t.Errorf("observations count = %d, want 2", len(merged.Observations))
		}
		// average completeness: (0.8 + 0.6) / 2 = 0.7
		if merged.Completeness != 0.7 {
			t.Errorf("completeness = %f, want 0.7", merged.Completeness)
		}
	})

	t.Run("empty_issues_zero_completeness", func(t *testing.T) {
		t.Parallel()
		// given
		chunks := []domain.ClusterScanResult{
			{Name: "empty", Observations: []string{"obs"}},
		}

		// when
		merged := domain.MergeClusterChunks("empty", chunks)

		// then
		if merged.Completeness != 0 {
			t.Errorf("completeness = %f, want 0", merged.Completeness)
		}
	})
}
