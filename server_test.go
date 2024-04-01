package dnist

import (
	"net/netip"
	"testing"
)

func TestMatchers(t *testing.T) {
	tests := []struct {
		name    string
		matcher routeMatcher
		req     request
		match   bool
	}{
		{
			name:    "domain/no-match",
			match:   false,
			matcher: &domainMatcher{domain: "example.com.", handler: handlerFunc(dropHandler)},
			req:     request{domain: "google.com."},
		},
		{
			name:    "domain/match",
			match:   true,
			matcher: &domainMatcher{domain: "example.com.", handler: handlerFunc(dropHandler)},
			req:     request{domain: "subdomain.example.com."},
		},
		{
			name:    "cidr/no-match",
			match:   false,
			matcher: &cidrMatcher{cidr: netip.MustParsePrefix("10.1.0.0/16"), handler: handlerFunc(dropHandler)},
			req:     request{client: netip.MustParseAddr("10.2.0.1")},
		},
		{
			name:    "cidr/match",
			match:   true,
			matcher: &cidrMatcher{cidr: netip.MustParsePrefix("10.0.0.0/8"), handler: handlerFunc(dropHandler)},
			req:     request{client: netip.MustParseAddr("10.2.0.1")},
		},
		{
			name:    "cidr/match-single",
			match:   true,
			matcher: &cidrMatcher{cidr: netip.MustParsePrefix("10.0.0.0/32"), handler: handlerFunc(dropHandler)},
			req:     request{client: netip.MustParseAddr("10.0.0.0")},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := test.matcher.Matches(test.req)
			if test.match && handler == nil {
				t.Fatal("should have matched route")
			} else if !test.match && handler != nil {
				t.Fatal("should not have matched route")
			}
		})
	}
}
