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

	// １行目の表示、文字列があふれる場合はスクロールする
	// display_buff = mes + "  " + mes であることを前提としている
	display_buff_len := len(display_buff)
	if display_buff_len <= 16 {
		oled.PrintWithPos(0, 0, display_buff)
	} else {
		oled.PrintWithPos(0, 0, display_buff[display_buff_pos:display_buff_pos+17])
		display_buff_pos++
		if display_buff_pos >= int16((display_buff_len/2)+1) {
			display_buff_pos = 0
		}
	}
}
