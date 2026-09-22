package main

import (
	"encoding/json"
	"testing"
)

// The cgo-free logic is tested here; scripts/smoke.py covers the C ABI boundary.

func TestApplyFieldsSetsPathsInStableOrder(t *testing.T) {
	body := []byte(`{"service_tier":"default","metadata":{"user":"u1"}}`)
	updated, err := applyFields(body, map[string]any{"service_tier": "priority", "store": false})
	if err != nil {
		t.Fatalf("applyFields: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(updated, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["service_tier"] != "priority" || got["store"] != false {
		t.Fatalf("unexpected body: %s", updated)
	}
	if got["metadata"].(map[string]any)["user"] != "u1" {
		t.Fatalf("unrelated fields must be preserved: %s", updated)
	}
}

func TestNormalizeRequestReturnsBodyUntouchedWithoutConfig(t *testing.T) {
	if err := configure([]byte(`{"config_yaml":""}`)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	request, err := json.Marshal(map[string]any{"Body": []byte(`{"a":1}`), "Model": "gpt-5.5"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw, err := normalizeRequest(request)
	if err != nil {
		t.Fatalf("normalizeRequest: %v", err)
	}
	assertEnvelopeBody(t, raw, `{"a":1}`)
}

func TestNormalizeRequestAppliesConfiguredFields(t *testing.T) {
	if err := configure([]byte(`{"config_yaml":"c2V0X2ZpZWxkczoKICBzZXJ2aWNlX3RpZXI6IHByaW9yaXR5Cg=="}`)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	request, err := json.Marshal(map[string]any{"Body": []byte(`{"model":"gpt-5.5"}`)})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw, err := normalizeRequest(request)
	if err != nil {
		t.Fatalf("normalizeRequest: %v", err)
	}
	assertEnvelopeBody(t, raw, `{"model":"gpt-5.5","service_tier":"priority"}`)
}

func assertEnvelopeBody(t *testing.T, raw []byte, want string) {
	t.Helper()
	var envelope struct {
		OK     bool `json:"ok"`
		Result struct {
			Body []byte `json:"Body"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if !envelope.OK {
		t.Fatalf("envelope not ok: %s", raw)
	}
	if string(envelope.Result.Body) != want {
		t.Fatalf("body = %q, want %q", envelope.Result.Body, want)
	}
}
