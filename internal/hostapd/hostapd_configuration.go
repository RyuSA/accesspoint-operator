package hostapd

import (
	"reflect"
	"strings"
)

type HostapdConfiguration struct {
	Networkinterface string `key:"interface"`
	Ssid             string `key:"ssid"`
	Password         string `key:"wpa_passphrase"`
	Bridge           string `key:"bridge"`
	Driver           string `key:"driver" default:"nl80211"`
	Mode             string `key:"hw_mode" default:"g"`
	CountryCode      string `key:"country_code" default:"JP"`
}

func ConfigureHostapd(h HostapdConfiguration) string {
	var builder strings.Builder
	join := func(key string, value string) string {
		return key + "=" + value + "\n"
	}

	t := reflect.TypeOf(h)
	v := reflect.ValueOf(h)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		key := field.Tag.Get("key")
		value := v.Field(i).String()
		if value == "" {
			value = field.Tag.Get("default")
			if value == "" {
				continue
			}
		}
		builder.WriteString(join(key, value))
	}
	defaultconfig := `ieee80211d=1
ieee80211n=1
wmm_enabled=1
macaddr_acl=0
auth_algs=1
ignore_broadcast_ssid=0
wpa=2
wpa_key_mgmt=WPA-PSK
rsn_pairwise=CCMP
ctrl_interface=/var/run/hostapd
ctrl_interface_group=0
`
	builder.WriteString(defaultconfig)
	return builder.String()
}
