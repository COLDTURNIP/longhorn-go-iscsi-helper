package util

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	lhnet "github.com/longhorn/go-common-libs/net"
)

func TestIPFamilyDecisionTable(t *testing.T) {
	for _, family := range []lhnet.IPFamily{lhnet.IPFamilyIPv4, lhnet.IPFamilyIPv6} {
		for _, storage := range []bool{false, true} {
			for _, podIP := range []string{"192.0.2.10", "2001:db8::10"} {
				for _, dualStack := range []bool{false, true} {
					name := fmt.Sprintf("%s/storage=%v/pod=%s/dual=%v", family, storage, podIP, dualStack)
					t.Run(name, func(t *testing.T) {
						t.Parallel()
						ifaces := []net.Interface{{Name: "other0"}, {Name: "pod0"}}
						addresses := map[string][]net.Addr{
							"other0": {&net.IPNet{IP: net.ParseIP("198.51.100.20")}, &net.IPNet{IP: net.ParseIP("2001:db8:2::20")}},
							"pod0":   {&net.IPNet{IP: net.ParseIP("127.0.0.1")}, &net.IPAddr{IP: net.ParseIP("fe80::1")}, &net.IPNet{IP: net.ParseIP(podIP)}},
						}
						if dualStack {
							addresses["pod0"] = append(addresses["pod0"], &net.IPAddr{IP: net.ParseIP("192.0.2.10")}, &net.IPNet{IP: net.ParseIP("2001:db8::10")})
						}
						if storage {
							// Put storage first and provide usable addresses of both families.
							ifaces = append([]net.Interface{{Name: "lhnet1"}}, ifaces...)
							addresses["lhnet1"] = []net.Addr{&net.IPNet{IP: net.ParseIP("198.51.100.10")}, &net.IPNet{IP: net.ParseIP("2001:db8:1::10")}}
						}
						ip, err := getIPToHostByFamily(family, podIP, func() ([]net.Interface, error) {
							return ifaces, nil
						}, func(iface net.Interface) ([]net.Addr, error) {
							if iface.Name == "lhnet1" {
								t.Fatal("storage interface addresses must never be consulted")
							}
							return addresses[iface.Name], nil
						})
						if dualStack || lhnet.IsUsableIPForFamily(net.ParseIP(podIP), family) {
							want := "192.0.2.10"
							if family == lhnet.IPFamilyIPv6 {
								want = "2001:db8::10"
							}
							if err != nil || ip != want {
								t.Fatalf("IP/error = %q/%v, want pod-network %q", ip, err, want)
							}
						} else if ip != "" || err == nil || !strings.Contains(err.Error(), "no usable "+string(family)+" address on POD_IP interface pod0") {
							t.Fatalf("IP/error = %q/%v, want strict missing-family error despite other/storage addresses", ip, err)
						}
					})
				}
			}
		}
	}
}

func TestIPFamilyErrors(t *testing.T) {
	interfaceErr := errors.New("interface enumeration failed")
	addressErr := errors.New("address enumeration failed")
	tests := []struct {
		name         string
		family       lhnet.IPFamily
		podIP        string
		ifaces       []net.Interface
		interfaceErr error
		addressErr   error
		wantErr      error
		wantText     string
	}{
		{name: "empty POD_IP with storage", family: lhnet.IPFamilyIPv4, ifaces: []net.Interface{{Name: "lhnet1"}}, wantText: "without POD_IP"},
		{name: "empty POD_IP IPv6", family: lhnet.IPFamilyIPv6, wantText: "without POD_IP"},
		{name: "invalid POD_IP", family: lhnet.IPFamilyIPv4, podIP: "invalid", wantText: "invalid POD_IP"},
		{name: "unusable POD_IP", family: lhnet.IPFamilyIPv6, podIP: "fe80::1", wantText: "invalid POD_IP"},
		{name: "invalid family", family: lhnet.IPFamily("bogus"), podIP: "192.0.2.10", wantText: "invalid explicit IP family"},
		{name: "interface error", family: lhnet.IPFamilyIPv4, podIP: "192.0.2.10", interfaceErr: interfaceErr, wantErr: interfaceErr},
		{name: "address error", family: lhnet.IPFamilyIPv6, podIP: "2001:db8::10", ifaces: []net.Interface{{Name: "pod0"}}, addressErr: addressErr, wantErr: addressErr},
		{name: "POD_IP interface absent", family: lhnet.IPFamilyIPv4, podIP: "192.0.2.10", ifaces: []net.Interface{{Name: "other0"}}, wantText: "cannot find pod-network interface"},
		{name: "POD_IP on storage is not pod context", family: lhnet.IPFamilyIPv4, podIP: "192.0.2.10", ifaces: []net.Interface{{Name: "lhnet1"}}, wantText: "cannot find pod-network interface"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ip, err := getIPToHostByFamily(tt.family, tt.podIP, func() ([]net.Interface, error) {
				if tt.podIP == "" || tt.wantText == "invalid POD_IP" || tt.wantText == "invalid explicit IP family" {
					t.Fatal("invalid or missing metadata must fail before interface enumeration")
				}
				return tt.ifaces, tt.interfaceErr
			}, func(iface net.Interface) ([]net.Addr, error) {
				if iface.Name == "lhnet1" {
					t.Fatal("storage interface must never be consulted")
				}
				return []net.Addr{&net.IPNet{IP: net.ParseIP("198.51.100.20")}}, tt.addressErr
			})
			if ip != "" || err == nil || (tt.wantErr != nil && !errors.Is(err, tt.wantErr)) || !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("IP/error = %q/%v, want %v containing %q", ip, err, tt.wantErr, tt.wantText)
			}
		})
	}
}

func TestIPFamilyPrimaryAddress(t *testing.T) {
	podIP := "2001:0db8:0:0:0:0:0:0010"
	ip, err := getIPToHostByFamily(lhnet.IPFamilyIPv6, podIP, func() ([]net.Interface, error) {
		return []net.Interface{{Name: "pod0"}}, nil
	}, func(net.Interface) ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.ParseIP("2001:db8::20")}, &net.IPAddr{IP: net.ParseIP(podIP)}}, nil
	})
	want := net.ParseIP(podIP).String()
	if err != nil || ip != want {
		t.Fatalf("IP/error = %q/%v, want canonical primary %q rather than another same-family address", ip, err, want)
	}
}

func TestIPFamilyUnspecified(t *testing.T) {
	for _, podIP := range []string{"192.0.2.10", "2001:0db8::10", "invalid", ""} {
		t.Run("POD_IP="+podIP, func(t *testing.T) {
			t.Setenv(lhnet.EnvPodIP, podIP)
			want, wantErr := GetIPToHost()
			ip, err := GetIPToHostByFamily(lhnet.IPFamilyUnspecified)
			if ip != want || (err == nil) != (wantErr == nil) || (err != nil && err.Error() != wantErr.Error()) {
				t.Fatalf("unspecified = %q/%v, GetIPToHost = %q/%v", ip, err, want, wantErr)
			}
		})
	}
}
