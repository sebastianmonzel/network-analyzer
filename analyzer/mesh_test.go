package analyzer

import "testing"

func TestMeshInfrastructureType(t *testing.T) {
	tests := []struct {
		name       string
		hostname   string
		wantType   string
		wantDevice bool
	}{
		{name: "repeater", hostname: "fritz-repeaterog", wantType: "repeater", wantDevice: true},
		{name: "powerline", hostname: "FRITZ!Powerline 1260", wantType: "powerline", wantDevice: true},
		{name: "power-line", hostname: "power-line bridge", wantType: "powerline", wantDevice: true},
		{name: "regular device", hostname: "living-room-tv", wantDevice: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotType, gotDevice := meshInfrastructureType(test.hostname)
			if gotType != test.wantType || gotDevice != test.wantDevice {
				t.Fatalf("meshInfrastructureType(%q) = %q, %t; want %q, %t",
					test.hostname, gotType, gotDevice, test.wantType, test.wantDevice)
			}
		})
	}
}

func TestAppendMeshClientsSkipsReservedInfrastructureIPs(t *testing.T) {
	repeater := &NetworkNode{IP: "192.168.178.38", DeviceType: "repeater"}
	seenIPs := map[string]bool{
		"192.168.178.38": true,
	}
	hosts := []*NetworkNode{
		{IP: "192.168.178.38", Name: "PC-192-168-178-38", DeviceType: "client", Active: true},
		{IP: "192.168.178.54", Name: "Phone", DeviceType: "client", Active: true},
	}

	appendMeshClients(repeater, hosts, seenIPs, true)

	if len(repeater.Children) != 1 || repeater.Children[0].IP != "192.168.178.54" {
		t.Fatalf("repeater children = %#v, want only client 192.168.178.54", repeater.Children)
	}
}
