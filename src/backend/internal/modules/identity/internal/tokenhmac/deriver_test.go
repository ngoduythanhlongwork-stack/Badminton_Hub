package tokenhmac

import "testing"

func TestDerivationIsDeterministicAndSecretBound(t *testing.T) {
	a, _ := New([]byte("12345678901234567890123456789012"))
	b, _ := New([]byte("abcdefghijklmnopqrstuvwxyzABCDEF"))
	tokenA, hashA, err := a.Derive("token", "account", "VERIFY_EMAIL")
	if err != nil {
		t.Fatal(err)
	}
	again, _, _ := a.Derive("token", "account", "VERIFY_EMAIL")
	tokenB, _, _ := b.Derive("token", "account", "VERIFY_EMAIL")
	if tokenA != again || tokenA == tokenB || string(hashA) == tokenA {
		t.Fatal("unexpected HMAC derivation behavior")
	}
}
