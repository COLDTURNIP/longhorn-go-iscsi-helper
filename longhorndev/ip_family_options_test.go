package longhorndev

import (
	"reflect"
	"testing"

	lhnet "github.com/longhorn/go-common-libs/net"

	"github.com/longhorn/go-iscsi-helper/types"
)

func TestDeviceIPFamilyOptions(t *testing.T) {
	creator := &LonghornDeviceCreator{}
	for _, family := range []lhnet.IPFamily{lhnet.IPFamilyUnspecified, lhnet.IPFamilyIPv4, lhnet.IPFamilyIPv6} {
		service, err := creator.NewDeviceWithOptions("volume", 4096, types.FrontendTGTBlockDev, 60, 15, 542, DeviceOptions{IPFamily: family})
		if err != nil {
			t.Fatal(err)
		}
		dev := service.(*LonghornDevice)
		if dev.options.IPFamily != family || dev.iscsiDeviceOptions().IPFamily != family {
			t.Fatalf("device/iSCSI family = %q/%q, want %q", dev.options.IPFamily, dev.iscsiDeviceOptions().IPFamily, family)
		}
		if dev.scsiTimeout != 60 || dev.iscsiAbortTimeout != 15 || dev.iscsiTargetRequestTimeout != 542 {
			t.Fatal("options changed existing timeouts")
		}
		if family == lhnet.IPFamilyUnspecified {
			legacy, err := creator.NewDevice("volume", 4096, types.FrontendTGTBlockDev, 60, 15, 542)
			if err != nil || !reflect.DeepEqual(service, legacy) {
				t.Fatalf("default constructor differs: %v", err)
			}
		}
	}
}
