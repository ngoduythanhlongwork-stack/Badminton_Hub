package argon2id

import "testing"

func TestHashRecordsParametersAndVerifies(t *testing.T) {
	h := Hasher{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}
	encoded, err := h.Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := h.Verify("correct horse battery staple", encoded)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	ok, err = h.Verify("wrong", encoded)
	if err != nil || ok {
		t.Fatalf("wrong password ok=%v err=%v", ok, err)
	}
}
