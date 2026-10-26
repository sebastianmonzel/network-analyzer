package analyzer

import "testing"

func TestManufacturerForMAC(t *testing.T) {
	tests := []struct {
		name string
		mac  string
		want string
	}{
		{name: "IEEE MA-L prefix", mac: "E0:06:30:12:34:56", want: "HUAWEI TECHNOLOGIES CO.,LTD"},
		{name: "IEEE MA-S prefix takes precedence", mac: "00:1B:C5:00:01:23", want: "Converging Systems Inc."},
		{name: "separator-independent parsing", mac: "50-e6-36-00-00-01", want: "AVM Audiovisuelles Marketing und Computersysteme GmbH"},
		{name: "locally administered randomized MAC", mac: "02:1B:C5:00:01:23"},
		{name: "multicast MAC", mac: "01:1B:C5:00:01:23"},
		{name: "invalid MAC", mac: "not-a-mac"},
		{name: "unknown globally administered prefix", mac: "04:00:00:00:00:01"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := manufacturerForMAC(test.mac); got != test.want {
				t.Fatalf("manufacturerForMAC(%q) = %q, want %q", test.mac, got, test.want)
			}
		})
	}
}
