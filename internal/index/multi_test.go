package index

import "testing"

func TestMultiContainsSpecificZone(t *testing.T) {
	multi := NewMulti()
	com := NewExact("com")
	com.Add("example.com")
	net := NewExact("net")
	net.Add("example.net")

	if err := multi.Register(net); err != nil {
		t.Fatalf("Register(net) error = %v", err)
	}
	if err := multi.Register(com); err != nil {
		t.Fatalf("Register(com) error = %v", err)
	}

	if !multi.Contains("com", "example.com") {
		t.Fatal("Contains(com, example.com) = false, want true")
	}
	if multi.Contains("net", "example.com") {
		t.Fatal("Contains(net, example.com) = true, want false")
	}
	if multi.Contains("org", "example.org") {
		t.Fatal("Contains(org, example.org) = true, want false for missing zone")
	}
}

func TestMultiZoneNamesDeterministicOrder(t *testing.T) {
	multi := NewMulti()
	com := NewExact("com")
	net := NewExact("net")
	org := NewExact("org")

	if err := multi.Register(org); err != nil {
		t.Fatalf("Register(org) error = %v", err)
	}
	if err := multi.Register(com); err != nil {
		t.Fatalf("Register(com) error = %v", err)
	}
	if err := multi.Register(net); err != nil {
		t.Fatalf("Register(net) error = %v", err)
	}

	got := multi.ZoneNames()
	want := []string{"com", "net", "org"}
	if len(got) != len(want) {
		t.Fatalf("len(ZoneNames()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ZoneNames()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLoadMulti(t *testing.T) {
	multi, err := LoadMulti(map[string]string{
		"net": fixturePath("small", "net.zone.slice"),
		"com": fixturePath("small", "com.zone"),
	})
	if err != nil {
		t.Fatalf("LoadMulti() error = %v", err)
	}

	if !multi.Contains("net", "example.net") {
		t.Fatal("Contains(net, example.net) = false, want true")
	}
	if multi.Contains("com", "example.net") {
		t.Fatal("Contains(com, example.net) = true, want false")
	}
}
