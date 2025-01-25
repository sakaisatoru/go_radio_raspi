package main

import (
	"fmt"
	"time"
)

func showclock() {
	mu.Lock()
	defer mu.Unlock()
	var s, s0 string
	var md byte
	
	if display_info == display_info_only_doubleheight_clock {
		n := time.Now()
		s = fmt.Sprintf("%2d%c%02d", n.Hour(),display_colon[colon],n.Minute())
		oled.SetDoubleHeight()
		//~ oled.PrintWithPos(6, 0, []byte(s))
		//~ oled.PrintUserNumericWithPos(6, []byte(s))
		oled.ShowClockWithUserfont(3, []byte(s))
		return
	} else {
		oled.SetNormal()
	}
	// alarm
	if clock_mode&1 == 1 {
		if (state_cdx == state_alarm_hour_set || state_cdx == state_alarm_min_set) && colon == 1 {
			if state_cdx == state_alarm_hour_set {
				s0 = fmt.Sprintf("A  :%02d", alarm_time.Minute()) // blink hour
			} else {
				s0 = fmt.Sprintf("A%2d:  ", alarm_time.Hour()) // blink min
			}
		} else {
			s0 = fmt.Sprintf("A%2d:%02d", alarm_time.Hour(),
				alarm_time.Minute())
		}
	} else {
		s0 = "      "
	}
	if md = 0x20; state_cdx == state_aux {
		md += 0x35 // 0x20+0x22 == U
	}
	if time.Since(display_volume_time) >= display_volume_time_span {
		display_volume = false
	}

	n := time.Now()
	if display_volume {
		s = fmt.Sprintf("vol:%2d   %c %2d%c%02d", volume,
			md, n.Hour(), display_colon[colon], n.Minute())
	} else {
		s = fmt.Sprintf("%s %c %c %2d%c%02d", s0,
			display_sleep[clock_mode&2],
			md, n.Hour(), display_colon[colon], n.Minute())
	}
	oled.PrintWithPos(0, 1, []byte(s))

	oled.PrintBuffer(0)
}
