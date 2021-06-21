package hostapd

import "strings"

type HostapdConfiguration struct {
	Networkinterface string
	Ssid             string
	Password         string
}

func (h *HostapdConfiguration) Configure() string {
	defaultconfig := `
interface=wlan0
driver=nl80211
bridge=br0
hw_mode=g
channel=6
ieee80211d=1
country_code=JP
ieee80211n=1
wmm_enabled=1
macaddr_acl=0
auth_algs=1
ignore_broadcast_ssid=0
wpa=2
wpa_passphrase=password
wpa_key_mgmt=WPA-PSK
rsn_pairwise=CCMP
ctrl_interface=/var/run/hostapd
ctrl_interface_group=0
`
	var builder strings.Builder
	join := func(key string, value string) string {
		return key + "=" + value + "\n"
	}
	builder.WriteString(defaultconfig)
	builder.WriteString(join("interface", h.Networkinterface))
	builder.WriteString(join("ssid", h.Ssid))
	builder.WriteString(join("wpa_passphrase", h.Password))
	return builder.String()
}
