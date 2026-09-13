package main

import (
	"errors"
	"net/url"
)

func (v *VPN) OpenExternalURL(address string) error {
	u, e := url.Parse(address)
	if e != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("Допустимы только веб-ссылки HTTP/HTTPS")
	}
	return v.app.Browser.OpenURL(u.String())
}
