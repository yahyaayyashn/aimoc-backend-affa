// Package blackvue berisi helper untuk bicara dengan dashcam BlackVue lewat tunnel
// Fleet Server: resolusi port CGI dinamis dan parsing skema URL blackvue://.
// Dipakai bersama oleh live-view proxy (internal/handler/camera_stream.go) dan
// backup VOD (internal/service/clip_backup.go) supaya logikanya tidak dobel.
package blackvue

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ProxyEntry struct {
	Conf struct {
		Name  string `json:"name"`
		Metas struct {
			DeviceMAC string `json:"device_mac"`
			DevicePSN string `json:"device_psn"`
		} `json:"metas"`
		RemotePort int `json:"remote_port"`
	} `json:"conf"`
	Status string `json:"status"`
}

type ProxyList struct {
	Proxies []ProxyEntry `json:"proxies"`
}

func fetchProxyList(fleetHost, fleetAPIPort string) (ProxyList, error) {
	apiURL := fmt.Sprintf("http://%s:%s/api/proxy/tcp", fleetHost, fleetAPIPort)
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return ProxyList{}, fmt.Errorf("gagal query Fleet Server: %w", err)
	}
	defer resp.Body.Close()

	var list ProxyList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return ProxyList{}, fmt.Errorf("gagal parse response Fleet Server: %w", err)
	}
	return list, nil
}

func resolvePort(fleetHost, fleetAPIPort, mac, psn, namePrefix string) (int, error) {
	list, err := fetchProxyList(fleetHost, fleetAPIPort)
	if err != nil {
		return 0, err
	}
	for _, p := range list.Proxies {
		if !strings.HasPrefix(p.Conf.Name, namePrefix) {
			continue
		}
		if p.Conf.Metas.DeviceMAC == mac || p.Conf.Metas.DevicePSN == psn {
			if p.Status != "online" {
				return 0, fmt.Errorf("dashcam sedang offline di Fleet Server")
			}
			return p.Conf.RemotePort, nil
		}
	}
	return 0, fmt.Errorf("device dengan MAC %s tidak terdaftar di Fleet Server", mac)
}

// ResolveCGIPort query Fleet Server API untuk port CGI terkini milik device (MAC/PSN).
// Port ini berubah setiap fleet-server di-restart sehingga tidak boleh pernah di-cache —
// pendekatan yang sama dipakai blackvue_capture.py di aimoc-ai-service.
func ResolveCGIPort(fleetHost, fleetAPIPort, mac, psn string) (int, error) {
	return resolvePort(fleetHost, fleetAPIPort, mac, psn, "CGI_")
}

// ResolveSocketPort query Fleet Server API untuk port command-socket (raw TCP, protokol
// port 9771) terkini milik device (MAC/PSN). Dipakai DeviceHealthPoller untuk cek SD
// card/RTC/GPS/SIM (lihat pkg/blackvue/socket.go) -- port ini juga berubah tiap restart
// fleet-server, sama seperti CGI.
func ResolveSocketPort(fleetHost, fleetAPIPort, mac, psn string) (int, error) {
	return resolvePort(fleetHost, fleetAPIPort, mac, psn, "Socket_")
}

// ParseStreamURL mem-parse skema blackvue://<mac>:<psn>@<fleet_host>:<fleet_api_port>/<mode>
// yang dipakai baik di Camera.StreamURL (backend) maupun CAMERAS env (aimoc-ai-service).
// mode adalah bagian path tanpa leading slash (mis. "live", "hss", atau "" untuk VOD).
func ParseStreamURL(streamURL string) (mac, psn, fleetHost, fleetAPIPort, mode string, err error) {
	u, err := url.Parse(streamURL)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("stream_url tidak valid: %w", err)
	}
	if u.Scheme != "blackvue" || u.User == nil {
		return "", "", "", "", "", fmt.Errorf("stream_url harus berskema blackvue://mac:psn@host:port/mode")
	}
	mac = u.User.Username()
	psn, _ = u.User.Password()
	fleetHost, fleetAPIPort, err = net.SplitHostPort(u.Host)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("stream_url tidak valid: host Fleet Server tidak lengkap: %w", err)
	}
	mode = strings.TrimPrefix(u.Path, "/")
	return mac, psn, fleetHost, fleetAPIPort, mode, nil
}
