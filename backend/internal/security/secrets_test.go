package security

import "testing"

func TestValidateSecretBlocksDefaultsInProduction(t *testing.T) {
	if err := ValidateSecret("JWT_SECRET", "changeme", 32, false); err == nil {
		t.Fatal("expected blocked default to be rejected in production")
	}
}

func TestValidateSecretAllowsDefaultsInDev(t *testing.T) {
	if err := ValidateSecret("JWT_SECRET", "changeme", 32, true); err != nil {
		t.Fatalf("expected dev mode to allow defaults: %v", err)
	}
}

func TestConstantTimeEqual(t *testing.T) {
	if !ConstantTimeEqual("secret", "secret") {
		t.Fatal("expected equal strings to match")
	}
	if ConstantTimeEqual("secret", "secrex") {
		t.Fatal("expected different strings to not match")
	}
}
