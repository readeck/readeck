package bookmarks

import (
	"net"

	"golang.org/x/net/idna"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/pkg/extract"
)

// checkIP will run the site IPs through the configured denied CIDR
// values. If there's a match, then it returns false and the matching rule.
func checkIP(ips []net.IP) (bool, string) {
	for _, cidr := range configs.Config.Extractor.DeniedIPs {
		for _, ip := range ips {
			if cidr.Contains(ip) {
				return false, cidr.String()
			}
		}
	}

	return true, ""
}

// CheckIPProcessor is a starting processor that resolves the
// ip of the link and checks if it's not denied.
func CheckIPProcessor(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepStart {
		return next
	}

	hostname := m.Extractor.Drop().URL.Hostname()
	host, err := idna.ToASCII(hostname)
	if err != nil {
		m.Cancel("invalid hostname %s", hostname)
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		m.Cancel("cannot resolve %s", host)
		return nil
	}

	if ok, rule := checkIP(ips); !ok {
		m.Cancel("ip %s blocked by rule %s", ips, rule)
		m.ResetContent()
		return nil
	}

	return next
}