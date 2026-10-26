package models

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// New and edited probes have explicit, bounded targets and cadence. Existing
// installations are not silently rewritten when this validation is introduced.
func ValidatePingTask(task PingTask) error {
	if strings.TrimSpace(task.Name) == "" || len(task.Name) > 128 || strings.ContainsAny(task.Name+task.Target, "\r\n\x00") || len(task.Target) > 2048 || task.Interval < 10 || task.Interval > 3600 || len(task.Clients) > 100 {
		return errors.New("probe requires a name, a valid target, 10–3600 second interval and at most 100 nodes")
	}
	seen := map[string]bool{}
	for _, id := range task.Clients {
		parsed, err := uuid.Parse(id)
		if err != nil || parsed == uuid.Nil || parsed.String() != id || seen[id] {
			return errors.New("invalid or duplicate probe node")
		}
		seen[id] = true
	}
	if !task.DefaultOn && len(task.Clients) == 0 {
		return errors.New("select at least one probe node")
	}
	switch task.Type {
	case "http":
		value, err := url.Parse(task.Target)
		if err != nil || value.Hostname() == "" || value.User != nil || value.Fragment != "" || (value.Scheme != "http" && value.Scheme != "https") {
			return errors.New("HTTP probe requires an HTTP(S) URL without credentials or fragment")
		}
	case "tcp":
		host, port, err := net.SplitHostPort(task.Target)
		if err != nil || host == "" {
			return errors.New("TCP target requires host:port; bracket IPv6 addresses")
		}
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return errors.New("invalid TCP port")
		}
	case "icmp":
		if task.Target == "" || strings.ContainsAny(task.Target, " /?#@") || strings.Contains(task.Target, ":") && net.ParseIP(task.Target) == nil {
			return errors.New("ICMP target requires an IP address or hostname")
		}
	default:
		return errors.New("probe type must be icmp, tcp or http")
	}
	return nil
}
