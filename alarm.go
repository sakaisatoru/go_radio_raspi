package main

import (
	"time"
)

func alarm_time_inc() {
	if state_cdx == state_alarm_hour_set {
		alarm_time = alarm_time.Add(1 * time.Hour)
	} else {
		alarm_time = alarm_time.Add(1 * time.Minute)
	}
}

func alarm_time_dec() {
	if state_cdx == state_alarm_min_set {
		alarm_time = alarm_time.Add(59 * time.Minute)
		// 時間が進んでしまうのでhourも補正する
	}
	alarm_time = alarm_time.Add(23 * time.Hour)
}
