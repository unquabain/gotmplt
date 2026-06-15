package main

import "testing"

func TestDig_FlatMap(t *testing.T) {
	data := map[string]any{"name": "alice", "age": 30}
	ref, found, remainder := dig(data, "name")
	if !found {
		t.Fatalf("expected found=true, remainder=%q", remainder)
	}
	got, ok := ref.Get()
	if !ok || got != "alice" {
		t.Errorf("Get() = %v, %v; want \"alice\", true", got, ok)
	}
}

func TestDig_NestedMap(t *testing.T) {
	data := map[string]any{
		"contact": map[string]any{
			"name": "alice",
			"address": map[string]any{
				"city": "Boston",
			},
		},
	}
	ref, found, remainder := dig(data, "contact.address.city")
	if !found {
		t.Fatalf("expected found=true, remainder=%q", remainder)
	}
	got, ok := ref.Get()
	if !ok || got != "Boston" {
		t.Errorf("Get() = %v, %v; want \"Boston\", true", got, ok)
	}
}

func TestDig_Slice(t *testing.T) {
	data := map[string]any{
		"items": []any{"a", "b", "c"},
	}
	ref, found, remainder := dig(data, "items.1")
	if !found {
		t.Fatalf("expected found=true, remainder=%q", remainder)
	}
	got, ok := ref.Get()
	if !ok || got != "b" {
		t.Errorf("Get() = %v, %v; want \"b\", true", got, ok)
	}
}

func TestDig_RefSetMutates(t *testing.T) {
	data := map[string]any{"name": "alice"}
	ref, found, _ := dig(data, "name")
	if !found {
		t.Fatal("expected found=true")
	}
	if err := ref.Set("bob"); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	if data["name"] != "bob" {
		t.Errorf("data[\"name\"] = %v, want \"bob\" (ref.Set must mutate original)", data["name"])
	}
}

func TestDig_MissingTopLevel(t *testing.T) {
	data := map[string]any{"name": "alice"}
	_, found, remainder := dig(data, "missing")
	if found {
		t.Errorf("expected found=false")
	}
	if remainder != "missing" {
		t.Errorf("remainder = %q, want %q", remainder, "missing")
	}
}

func TestDig_MissingNested(t *testing.T) {
	data := map[string]any{
		"contact": map[string]any{"name": "alice"},
	}
	_, found, remainder := dig(data, "contact.missing.deep")
	if found {
		t.Errorf("expected found=false")
	}
	if remainder != "missing.deep" {
		t.Errorf("remainder = %q, want %q", remainder, "missing.deep")
	}
}

func TestDig_SliceOutOfRange(t *testing.T) {
	data := map[string]any{"items": []any{"a"}}
	_, found, remainder := dig(data, "items.5")
	if found {
		t.Errorf("expected found=false for out-of-range index")
	}
	if remainder != "5" {
		t.Errorf("remainder = %q, want %q", remainder, "5")
	}
}

func TestDigSet_ExistingKey(t *testing.T) {
	data := map[string]any{"name": "alice"}
	if err := digSet(data, "name", "bob"); err != nil {
		t.Fatalf("digSet error: %v", err)
	}
	if data["name"] != "bob" {
		t.Errorf("data[\"name\"] = %v, want \"bob\"", data["name"])
	}
}

func TestDigSet_ExistingNestedKey(t *testing.T) {
	data := map[string]any{
		"contact": map[string]any{"name": "alice"},
	}
	if err := digSet(data, "contact.name", "bob"); err != nil {
		t.Fatalf("digSet error: %v", err)
	}
	got := data["contact"].(map[string]any)["name"]
	if got != "bob" {
		t.Errorf("contact.name = %v, want \"bob\"", got)
	}
}

func TestDigSet_NewTopLevelKey(t *testing.T) {
	data := map[string]any{"name": "alice"}
	if err := digSet(data, "city", "Boston"); err != nil {
		t.Fatalf("digSet error: %v", err)
	}
	if data["city"] != "Boston" {
		t.Errorf("data[\"city\"] = %v, want \"Boston\"", data["city"])
	}
}

func TestDigSet_NewNestedPath(t *testing.T) {
	data := map[string]any{}
	if err := digSet(data, "a.b.c", "deep"); err != nil {
		t.Fatalf("digSet error: %v", err)
	}
	a, ok := data["a"].(map[string]any)
	if !ok {
		t.Fatalf("data[\"a\"] = %v (%T), want map[string]any", data["a"], data["a"])
	}
	b, ok := a["b"].(map[string]any)
	if !ok {
		t.Fatalf("a[\"b\"] = %v (%T), want map[string]any", a["b"], a["b"])
	}
	if b["c"] != "deep" {
		t.Errorf("a.b.c = %v, want \"deep\"", b["c"])
	}
}

func TestDigSet_PartialExistingPath(t *testing.T) {
	data := map[string]any{
		"contact": map[string]any{"name": "alice"},
	}
	if err := digSet(data, "contact.address.city", "Boston"); err != nil {
		t.Fatalf("digSet error: %v", err)
	}
	contact := data["contact"].(map[string]any)
	if contact["name"] != "alice" {
		t.Errorf("existing key clobbered: name = %v", contact["name"])
	}
	address, ok := contact["address"].(map[string]any)
	if !ok {
		t.Fatalf("address not created: %v (%T)", contact["address"], contact["address"])
	}
	if address["city"] != "Boston" {
		t.Errorf("address.city = %v, want \"Boston\"", address["city"])
	}
}
