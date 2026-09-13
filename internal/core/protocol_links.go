package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"net/url"
	"strconv"
	"strings"
)

func serverFromOptions(p map[string]any) (Server, error) {
	data, e := yaml.Marshal(map[string]any{"proxies": []any{p}})
	if e != nil {
		return Server{}, e
	}
	servers, e := ParseSubscription(data, "")
	if e != nil {
		return Server{}, e
	}
	return servers[0], nil
}
func parseExtendedLink(raw string) (Server, error) {
	if strings.HasPrefix(raw, "vmess://") {
		data, e := decodeBase64(strings.TrimPrefix(raw, "vmess://"))
		if e != nil {
			return Server{}, e
		}
		var v map[string]any
		if json.Unmarshal(data, &v) != nil {
			return Server{}, errors.New("VMess: ожидается Base64 JSON")
		}
		str := func(k string) string {
			if v[k] == nil {
				return ""
			}
			return fmt.Sprint(v[k])
		}
		port, e := strconv.Atoi(str("port"))
		if e != nil {
			return Server{}, errors.New("VMess: неверный порт")
		}
		aid := 0
		if str("aid") != "" {
			aid, e = strconv.Atoi(str("aid"))
			if e != nil {
				return Server{}, errors.New("VMess: неверный alterId")
			}
		}
		cipher := str("scy")
		if cipher == "" {
			cipher = "auto"
		}
		p := map[string]any{"name": str("ps"), "type": "vmess", "server": str("add"), "port": port, "uuid": str("id"), "alterId": aid, "cipher": cipher, "tls": str("tls") == "tls", "servername": str("sni")}
		if str("alpn") != "" {
			p["alpn"] = strings.Split(str("alpn"), ",")
		}
		if str("fp") != "" {
			p["client-fingerprint"] = str("fp")
		}
		switch str("net") {
		case "", "tcp":
			if str("type") != "" && str("type") != "none" {
				return Server{}, errors.New("VMess TCP header импортируйте через YAML")
			}
		case "ws":
			p["network"] = "ws"
			p["ws-opts"] = map[string]any{"path": str("path"), "headers": map[string]string{"Host": str("host")}}
		case "grpc":
			p["network"] = "grpc"
			p["grpc-opts"] = map[string]string{"grpc-service-name": str("path")}
		default:
			return Server{}, errors.New("VMess: этот транспорт импортируйте через YAML")
		}
		return serverFromOptions(p)
	}
	u, e := url.Parse(raw)
	if e != nil || u.User == nil || u.Hostname() == "" {
		return Server{}, errors.New("Некорректная ссылка")
	}
	port := 443
	if u.Port() != "" {
		port, e = strconv.Atoi(u.Port())
		if e != nil {
			return Server{}, e
		}
	}
	q := u.Query()
	p := map[string]any{"name": u.Fragment, "type": u.Scheme, "server": u.Hostname(), "port": port, "sni": q.Get("sni")}
	if q.Get("alpn") != "" {
		p["alpn"] = strings.Split(q.Get("alpn"), ",")
	}
	if u.Scheme == "tuic" {
		password, ok := u.User.Password()
		if !ok {
			return Server{}, errors.New("TUIC v5: нужны UUID и пароль")
		}
		p["uuid"] = u.User.Username()
		p["password"] = password
		for _, k := range []string{"congestion_control", "udp_relay_mode"} {
			if q.Get(k) != "" {
				dest := strings.ReplaceAll(k, "_", "-")
				if k == "congestion_control" {
					dest = "congestion-controller"
				}
				p[dest] = q.Get(k)
			}
		}
	} else {
		p["password"] = u.User.Username()
	}
	for _, k := range []string{"insecure", "allowInsecure", "skip-cert-verify"} {
		if q.Get(k) != "" {
			v, e := strconv.ParseBool(q.Get(k))
			if e != nil {
				return Server{}, errors.New("Некорректный параметр проверки сертификата")
			}
			p["skip-cert-verify"] = v
		}
	}
	return serverFromOptions(p)
}

func extendedTransport(raw string, s Server, q url.Values) (Server, error) {
	p, e := s.Proxy()
	if e != nil {
		return Server{}, e
	}
	if q.Get("type") == "xhttp" {
		if s.Protocol != "VLESS" {
			return Server{}, errors.New("XHTTP поддерживается только для VLESS")
		}
		if q.Get("extra") != "" {
			return Server{}, errors.New("XHTTP extra: импортируйте Mihomo YAML, чтобы сохранить расширенные параметры")
		}
		mode := q.Get("mode")
		if mode == "" {
			mode = "auto"
		}
		switch mode {
		case "auto", "stream-one", "stream-up", "packet-up":
		default:
			return Server{}, errors.New("Некорректный режим XHTTP")
		}
		p["network"] = "xhttp"
		p["xhttp-opts"] = map[string]string{"path": q.Get("path"), "host": q.Get("host"), "mode": mode}
	} else {
		p["network"] = "grpc"
		p["grpc-opts"] = map[string]string{"grpc-service-name": q.Get("serviceName")}
	}
	p["name"] = s.Name
	if q.Get("encryption") != "" && q.Get("encryption") != "none" {
		p["encryption"] = q.Get("encryption")
	}
	return serverFromOptions(p)
}
