package main

import (
	"fmt"
	"local.packages/mpvctl"
	"time"
)

func show_volume() {
	mu.Lock()
	defer mu.Unlock()

	s := fmt.Sprintf("vol:%2d", volume)
	oled.PrintWithPos(0, 1, []byte(s))

	display_volume_time = time.Now()
	display_volume = true
}

func inc_volume() {
	volume++
	if volume > mpvctl.Volume_max {
		volume = mpvctl.Volume_max
	}
	mpvctl.Setvol(volume)
	show_volume()
}

func dec_volume() {
	volume--
	if volume < mpvctl.Volume_min {
		volume = mpvctl.Volume_min
	}
	mpvctl.Setvol(volume)
	show_volume()
}
