package core

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

func parseWireGuard(raw string) (Server, error) {
	p := map[string]any{"name": "WireGuard", "type": "wireguard"}
	awg := map[string]any{}
	section := ""
	peers := 0
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = line
			if section == "[Peer]" {
				peers++
			}
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return Server{}, errors.New("Некорректная строка WireGuard")
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.TrimSpace(v)
		switch section {
		case "[Interface]":
			switch k {
			case "privatekey":
				p["private-key"] = v
			case "address":
				for _, a := range strings.Split(v, ",") {
					ip := strings.Split(strings.TrimSpace(a), "/")[0]
					if net.ParseIP(ip) == nil {
						return Server{}, errors.New("Некорректный IP WireGuard")
					}
					if strings.Contains(ip, ":") {
						p["ipv6"] = ip
					} else {
						p["ip"] = ip
					}
				}
			case "dns":
				values := []string{}
				for _, d := range strings.Split(v, ",") {
					values = append(values, strings.TrimSpace(d))
				}
				p["dns"] = values
				p["remote-dns-resolve"] = true
			case "mtu":
				n, e := strconv.Atoi(v)
				if e != nil {
					return Server{}, e
				}
				p["mtu"] = n
			case "jc", "jmin", "jmax", "s1", "s2", "s3", "s4":
				n, e := strconv.Atoi(v)
				if e != nil {
					return Server{}, e
				}
				awg[k] = n
			case "h1", "h2", "h3", "h4", "i1", "i2", "i3", "i4", "i5":
				awg[k] = v
			default:
				return Server{}, errors.New("Неподдерживаемый параметр WireGuard: " + k)
			}
		case "[Peer]":
			switch k {
			case "publickey":
				p["public-key"] = v
			case "presharedkey":
				p["pre-shared-key"] = v
			case "endpoint":
				host, port, e := net.SplitHostPort(v)
				if e != nil {
					return Server{}, e
				}
				n, e := strconv.Atoi(port)
				if e != nil {
					return Server{}, e
				}
				p["server"] = host
				p["port"] = n
			case "allowedips":
				values := []string{}
				for _, cidr := range strings.Split(v, ",") {
					values = append(values, strings.TrimSpace(cidr))
				}
				p["allowed-ips"] = values
			case "persistentkeepalive":
				n, e := strconv.Atoi(v)
				if e != nil {
					return Server{}, e
				}
				p["persistent-keepalive"] = n
			default:
				return Server{}, errors.New("Неподдерживаемый параметр peer: " + k)
			}
		default:
			return Server{}, errors.New("Нужна секция Interface или Peer")
		}
	}
	if peers != 1 {
		return Server{}, errors.New("Импорт WireGuard поддерживает один Peer")
	}
	if len(awg) > 0 {
		return Server{}, errors.New("Версия AmneziaWG неоднозначна: импортируйте Mihomo YAML с amnezia-wg-option.version")
	}
	return serverFromOptions(p)
}
