package agentauth

import (
	"testing"
	"time"
)

func TestMintVerify(t *testing.T) {
	token, err := Mint("secret", Claims{TenantID: "rivera", CallID: "call-1"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := Verify("secret", token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.TenantID != "rivera" || claims.CallID != "call-1" {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	token, err := Mint("secret", Claims{
		TenantID: "rivera", CallID: "call-1",
		ExpiresAt: time.Now().Add(-time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify("secret", token); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestWrongSecretRejected(t *testing.T) {
	token, err := Mint("secret", Claims{TenantID: "rivera", CallID: "call-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify("other", token); err == nil {
		t.Fatal("token verified with the wrong secret")
	}
}

func TestBootstrapKey(t *testing.T) {
	if !BootstrapKey("abc", "abc") {
		t.Fatal("matching key rejected")
	}
	if BootstrapKey("abc", "abd") {
		t.Fatal("mismatched key accepted")
	}
	if BootstrapKey("", "") {
		t.Fatal("empty key accepted")
	}
}
