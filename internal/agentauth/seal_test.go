package agentauth

import "testing"

func TestSealRoundTrip(t *testing.T) {
	type payload struct {
		CallID string `json:"call_id"`
		Phone  string `json:"phone"`
	}
	token, err := Seal("secret", payload{CallID: "c1", Phone: "9876543210"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got payload
	if err := Open("secret", token, &got); err != nil {
		t.Fatal(err)
	}
	if got.CallID != "c1" || got.Phone != "9876543210" {
		t.Fatalf("payload = %+v", got)
	}
}

func TestSealRejectsTampering(t *testing.T) {
	token, err := Seal("secret", map[string]string{"call_id": "c1"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := Open("other", token, &map[string]string{}); err == nil {
		t.Fatal("opened with wrong secret")
	}
}
