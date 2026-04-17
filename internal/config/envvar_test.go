package config

import "testing"

func TestValidateEnvVarName(t *testing.T) {
	t.Parallel()

	valid := []string{"OPENAI_API_KEY", "A", "_TOKEN", "KEY_123"}
	for _, name := range valid {
		if err := ValidateEnvVarName(name); err != nil {
			t.Fatalf("ValidateEnvVarName(%q) error = %v", name, err)
		}
	}

	invalid := []string{"", " ", "BAD KEY", "1START", "A-B", "A.B", "A=B"}
	for _, name := range invalid {
		if err := ValidateEnvVarName(name); err == nil {
			t.Fatalf("ValidateEnvVarName(%q) expected error", name)
		}
	}
}

func TestNormalizeEnvVarNameForCompare(t *testing.T) {
	t.Parallel()

	if got := NormalizeEnvVarNameForCompare(" OPENAI_API_KEY "); got == "" {
		t.Fatal("NormalizeEnvVarNameForCompare() should not return empty for valid key")
	}
}

func TestIsProtectedEnvVarName(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"PATH",
		"path",
		"Home",
		"USERPROFILE",
		"comspec",
		"LD_LIBRARY_PATH",
	} {
		if !IsProtectedEnvVarName(name) {
			t.Fatalf("expected %q to be protected", name)
		}
	}
	if IsProtectedEnvVarName("OPENAI_API_KEY") {
		t.Fatal("OPENAI_API_KEY should not be protected")
	}
}
