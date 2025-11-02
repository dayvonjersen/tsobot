package main

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

func wttr(loc string, freedom bool) string {
	loc = strings.TrimSpace(loc)
	loc = url.PathEscape(loc)
	if loc == "" {
		return "usage: .w [location]"
	}
	resp, err := http.Get("http://wttr.in/" + loc + "?format=2" + func() string {
		if freedom {
			return ""
		}
		return "&u"
	}())
	checkErr(err)
	if resp.StatusCode != 200 {
		printResponse(resp)
		return resp.Status
	}
	body, _ := ioutil.ReadAll(resp.Body)
	return string(body)
}
