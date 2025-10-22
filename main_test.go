package main

import "testing"

func TestParseImageBase(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantOwner   string
		wantPackage string
		wantErr     bool
	}{
		{
			name:        "simple package",
			input:       "ghcr.io/myoung34/policies",
			wantOwner:   "myoung34",
			wantPackage: "policies",
			wantErr:     false,
		},
		{
			name:        "nested package",
			input:       "ghcr.io/myoung34/kyverno-test/policies",
			wantOwner:   "myoung34",
			wantPackage: "kyverno-test/policies",
			wantErr:     false,
		},
		{
			name:        "package with tag",
			input:       "ghcr.io/myoung34/kyverno-test/policies:v0.0.1",
			wantOwner:   "myoung34",
			wantPackage: "kyverno-test/policies",
			wantErr:     false,
		},
		{
			name:        "deeply nested package with tag",
			input:       "ghcr.io/foo/bar/baz/qux:latest",
			wantOwner:   "foo",
			wantPackage: "bar/baz/qux",
			wantErr:     false,
		},
		{
			name:        "package with digest",
			input:       "ghcr.io/owner/package:sha256-abcd1234",
			wantOwner:   "owner",
			wantPackage: "package",
			wantErr:     false,
		},
		{
			name:        "invalid format - no slashes",
			input:       "invalid",
			wantOwner:   "",
			wantPackage: "",
			wantErr:     true,
		},
		{
			name:        "invalid format - missing package",
			input:       "ghcr.io/owner",
			wantOwner:   "",
			wantPackage: "",
			wantErr:     true,
		},
		{
			name:        "invalid format - only one slash",
			input:       "ghcr.io/owner/",
			wantOwner:   "",
			wantPackage: "",
			wantErr:     true,
		},
		{
			name:        "empty string",
			input:       "",
			wantOwner:   "",
			wantPackage: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, pkg, err := parseImageBase(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseImageBase(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}

			if err == nil {
				if owner != tt.wantOwner {
					t.Errorf("parseImageBase(%q) owner = %q, want %q", tt.input, owner, tt.wantOwner)
				}
				if pkg != tt.wantPackage {
					t.Errorf("parseImageBase(%q) package = %q, want %q", tt.input, pkg, tt.wantPackage)
				}
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "replace colons",
			input: "v0.0.1:latest",
			want:  "v0.0.1_latest",
		},
		{
			name:  "replace slashes",
			input: "owner/package",
			want:  "owner_package",
		},
		{
			name:  "replace both",
			input: "owner/package:v0.0.1",
			want:  "owner_package_v0.0.1",
		},
		{
			name:  "no special chars",
			input: "simple",
			want:  "simple",
		},
		{
			name:  "multiple colons and slashes",
			input: "a/b/c:d:e",
			want:  "a_b_c_d_e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizePath(tt.input)
			if got != tt.want {
				t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		setEnv       bool
		want         string
	}{
		{
			name:         "env var set",
			key:          "TEST_VAR_1",
			defaultValue: "default",
			envValue:     "custom",
			setEnv:       true,
			want:         "custom",
		},
		{
			name:         "env var not set",
			key:          "TEST_VAR_2",
			defaultValue: "default",
			envValue:     "",
			setEnv:       false,
			want:         "default",
		},
		{
			name:         "env var set to empty string",
			key:          "TEST_VAR_3",
			defaultValue: "default",
			envValue:     "",
			setEnv:       true,
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.key, tt.envValue)
			}

			got := getEnvOrDefault(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvOrDefault(%q, %q) = %q, want %q", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestGetEnvAsIntOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		setEnv       bool
		want         int
	}{
		{
			name:         "valid integer",
			key:          "TEST_INT_1",
			defaultValue: 10,
			envValue:     "42",
			setEnv:       true,
			want:         42,
		},
		{
			name:         "env var not set",
			key:          "TEST_INT_2",
			defaultValue: 10,
			envValue:     "",
			setEnv:       false,
			want:         10,
		},
		{
			name:         "invalid integer",
			key:          "TEST_INT_3",
			defaultValue: 10,
			envValue:     "not-a-number",
			setEnv:       true,
			want:         10,
		},
		{
			name:         "zero value",
			key:          "TEST_INT_4",
			defaultValue: 10,
			envValue:     "0",
			setEnv:       true,
			want:         0,
		},
		{
			name:         "negative integer",
			key:          "TEST_INT_5",
			defaultValue: 10,
			envValue:     "-5",
			setEnv:       true,
			want:         -5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.key, tt.envValue)
			}

			got := getEnvAsIntOrDefault(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvAsIntOrDefault(%q, %d) = %d, want %d", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}
