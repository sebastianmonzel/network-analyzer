package analyzer

import "testing"

func TestCallerIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
		wantErr    bool
	}{
		{name: "IPv4 with port", remoteAddr: "192.0.2.10:8080", want: "192.0.2.10"},
		{name: "IPv6 with port", remoteAddr: "[2001:db8::10]:8080", want: "2001:db8::10"},
		{name: "IPv4-mapped IPv6", remoteAddr: "[::ffff:192.0.2.10]:8080", want: "192.0.2.10"},
		{name: "invalid address", remoteAddr: "not-an-ip:8080", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := callerIP(test.remoteAddr)
			if (err != nil) != test.wantErr {
				t.Fatalf("callerIP() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("callerIP() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCallerIPResolvesLoopbackToLocalAddress(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
	}{
		{name: "IPv4 loopback", remoteAddr: "127.0.0.1:8080"},
		{name: "IPv6 loopback", remoteAddr: "[::1]:8080"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := callerIPWithLocalAddress(test.remoteAddr, func() (string, error) {
				return "192.0.2.20", nil
			})
			if err != nil {
				t.Fatalf("callerIPWithLocalAddress() error = %v", err)
			}
			if got != "192.0.2.20" {
				t.Fatalf("callerIPWithLocalAddress() = %q, want %q", got, "192.0.2.20")
			}
		})
	}
}

func TestSameIP(t *testing.T) {
	if !sameIP("192.0.2.10", "::ffff:192.0.2.10") {
		t.Fatal("sameIP() should match equivalent IPv4 and IPv4-mapped IPv6 addresses")
	}
	if sameIP("192.0.2.10", "192.0.2.11") {
		t.Fatal("sameIP() matched different addresses")
	}
}

func TestFindMeshPath(t *testing.T) {
	root := &NetworkNode{
		Name:       "Router",
		IP:         "192.0.2.1",
		DeviceType: "router",
		Active:     true,
		Children: []*NetworkNode{
			{
				Name:       "Repeater",
				IP:         "192.0.2.2",
				DeviceType: "repeater",
				Active:     true,
				Children: []*NetworkNode{
					{Name: "Client", IP: "192.0.2.20", DeviceType: "host", Active: true},
				},
			},
		},
	}

	path := findMeshPath(root, "192.0.2.20")
	if len(path) != 3 {
		t.Fatalf("findMeshPath() returned %d nodes, want 3", len(path))
	}
	for i, want := range []string{"Router", "Repeater", "Client"} {
		if path[i].Name != want {
			t.Errorf("path[%d].Name = %q, want %q", i, path[i].Name, want)
		}
	}

	if got := findMeshPath(root, "192.0.2.99"); got != nil {
		t.Errorf("findMeshPath() for unknown IP = %v, want nil", got)
	}
	if got := findMeshPath(nil, "192.0.2.20"); got != nil {
		t.Errorf("findMeshPath() for nil tree = %v, want nil", got)
	}
}
