package client

import (
	"net"

	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/utils/geoip"
)

func getClientIPType(ip net.IP) int {
	// 0:ipv4 1:ipv6 -1:错误的输入
	if ip == nil {
		return -1
	}
	if ip.To4() == nil {
		return 1
	}
	return 0
}

func saveClientBasicInfo(info map[string]interface{}, uuid string, fallbackIP string) error {
	// Only machine inventory belongs to the reporting agent. Tokens, visibility,
	// names, billing and notes remain exclusively controller-owned.
	filtered := map[string]interface{}{"uuid": uuid}
	for _, key := range []string{"cpu_name", "virtualization", "arch", "cpu_cores", "cpu_physical_cores", "os", "kernel_version", "gpu_name", "ipv4", "ipv6", "mem_total", "swap_total", "disk_total", "version"} {
		if value, ok := info[key]; ok {
			filtered[key] = value
		}
	}
	applyFallbackClientIP(filtered, fallbackIP)
	appendClientRegionFromGeoIP(filtered)
	return clients.SaveClientInfo(filtered)
}

func applyFallbackClientIP(info map[string]interface{}, fallbackIP string) {
	if hasClientIP(info) {
		return
	}
	ip := net.ParseIP(fallbackIP)

	switch getClientIPType(ip) {
	case 0:
		info["ipv4"] = fallbackIP
	case 1:
		info["ipv6"] = fallbackIP
	}
}

func hasClientIP(info map[string]interface{}) bool {
	if ipv4, ok := info["ipv4"].(string); ok && ipv4 != "" {
		return true
	}
	if ipv6, ok := info["ipv6"].(string); ok && ipv6 != "" {
		return true
	}
	return false
}

func appendClientRegionFromGeoIP(info map[string]interface{}) {
	cfg, err := config.GetAs[bool](config.GeoIpEnabledKey)
	if err != nil || !cfg {
		return
	}

	for _, key := range []string{"ipv4", "ipv6"} {
		ipStr, ok := info[key].(string)
		if !ok || ipStr == "" {
			continue
		}
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		record, _ := geoip.GetGeoInfo(ip)
		if record == nil {
			continue
		}
		region := geoip.GetRegionUnicodeEmoji(record.ISOCode)
		if region == "" {
			continue
		}
		info["region"] = region
		return
	}
}
