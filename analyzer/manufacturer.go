package analyzer

import (
	"bufio"
	_ "embed"
	"fmt"
	"net"
	"strings"
)

//go:embed oui.tsv
var ouiRegistry string

var ouiVendors = loadOUIRegistry()

func loadOUIRegistry() map[string]string {
	vendors := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(ouiRegistry))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		prefix, vendor, found := strings.Cut(scanner.Text(), "\t")
		if !found || (len(prefix) != 6 && len(prefix) != 7 && len(prefix) != 9) || vendor == "" {
			panic(fmt.Sprintf("invalid embedded IEEE OUI registry at line %d", lineNumber))
		}
		vendors[prefix] = vendor
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	return vendors
}

func manufacturerForMAC(address string) string {
	mac, err := net.ParseMAC(strings.TrimSpace(address))
	if err != nil || len(mac) != 6 {
		return ""
	}

	if mac[0]&0x03 != 0 {
		return ""
	}

	hexAddress := strings.ToUpper(strings.ReplaceAll(mac.String(), ":", ""))
	for _, prefixLength := range []int{9, 7, 6} {
		if vendor, ok := ouiVendors[hexAddress[:prefixLength]]; ok {
			return vendor
		}
	}
	return ""
}
