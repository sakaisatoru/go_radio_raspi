package main

import (
	"github.com/stianeikeland/go-rpio/v4"
	"local.packages/mpvctl"
	"local.packages/netradio"
	"log"
	"strings"
)

func tune() {
	var (
		station_url string
		err         error = nil
	)
	radio_enable = false
	display_info = display_info_default
	mpv_infovalue = stlist[pos].Name
	infoupdate(0, mpv_infovalue)

	args := strings.Split(stlist[pos].Url, "/")
	if args[0] == "plugin:" {
		switch args[1] {
		case "afn.py":
			station_url, err = netradio.AFN_get_url_with_api(args[2])
		case "radiko.py":
			station_url, err = netradio.Radiko_get_url(args[2])
		default:
			break
		}
		if err != nil {
			mpv_infovalue = errmessage[ERROR_TUNING]
			infoupdate(0, mpv_infovalue)
			log.Println("tune() ", err)
			return
		}
	} else {
		station_url = stlist[pos].Url
	}
	mpvctl.Setvol(volume)

	mpvctl.Loadfile(station_url)
	rpio.Pin(23).High() // AF amp enable
	radio_enable = true
}

func next_tune() {
	if radio_enable == true {
		if pos < stlen-1 {
			pos++
		}
	}
	tune()
}

func prior_tune() {
	if radio_enable == true {
		if pos > 0 {
			pos--
		}
	}
	tune()
}

func toggle_radio() {
	if radio_enable {
		mpvctl.Stop()
	} else {
		tune()
	}
}
