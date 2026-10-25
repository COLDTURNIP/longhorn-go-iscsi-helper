package iscsidev

import (
	"errors"
	"testing"

	lhnet "github.com/longhorn/go-common-libs/net"
)

func TestInitiatorIPFamilyOptions(t *testing.T) {
	for _, family := range []lhnet.IPFamily{lhnet.IPFamilyUnspecified, lhnet.IPFamilyIPv4, lhnet.IPFamilyIPv6} {
		dev := newDevice("volume", "/socket", "longhorn", "size=4096", 60, 15, DeviceOptions{IPFamily: family}, nil)
		if dev.options.IPFamily != family || dev.ScsiTimeout != 60 || dev.IscsiAbortTimeout != 15 {
			t.Fatalf("constructor lost options: %+v", dev)
		}
		for _, portal := range []string{"192.0.2.10", "2001:db8::10", ""} {
			selected := false
			ip, err := dev.initiatorIPWithSelectors(func(target string) (string, error) {
				if target != GetTargetName("volume") {
					t.Fatalf("unexpected target %q", target)
				}
				return portal, nil
			}, func(got lhnet.IPFamily) (string, error) {
				selected = true
				if got != family {
					t.Fatalf("fallback family = %q, want %q", got, family)
				}
				return "new-portal", nil
			})
			want := portal
			if want == "" {
				want = "new-portal"
			}
			if err != nil || ip != want || selected != (portal == "") {
				t.Fatalf("family %q portal %q: IP/error/selected = %q/%v/%v", family, portal, ip, err, selected)
			}
		}
		selected := false
		ip, err := dev.initiatorIPWithSelectors(func(string) (string, error) {
			return "", errors.New("session lookup failed")
		}, func(got lhnet.IPFamily) (string, error) {
			selected = true
			if got != family {
				t.Fatalf("fallback family after session error = %q, want %q", got, family)
			}
			return "new-portal", nil
		})
		if err != nil || ip != "new-portal" || !selected {
			t.Fatalf("family %q session error: IP/error/selected = %q/%v/%v, want family fallback", family, ip, err, selected)
		}
	}
}
