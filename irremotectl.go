package main

import (
	"local.packages/irremote"
	"time"
)

//~ 	Ir_Power    = 0x10d8
//~ 	Ir_A        = 0x10f8
//~ Ir_B        = 0x1078
//~ 	Ir_C        = 0x1058
//~ 	Ir_N        = 0x10A0
//~ Ir_NE       = 0x1021
//~ Ir_E        = 0x1080
//~ Ir_SE       = 0x1081
//~ 	Ir_S        = 0x1000
//~ 	Ir_SW       = 0x1011
//~ Ir_W        = 0x1010
//~ 	Ir_NW       = 0x10B1
//~ 	Ir_Center   = 0x1020
//~ Ir_Holdflag = 0x10000

var (
	irrepeat_time time.Time
	irrepeat_on   bool = false
	irfunc             = map[int32]func(){
		irremote.Ir_A: func() {
			// OLED１行目の表示切り替え
			display_info++
			if display_info >= display_info_end {
				display_info = display_info_default
			}
			switch display_info {
			case display_info_default:
				// デフォルトではバッファ(mpv_infovalue)を表示する。
				// バッファは大抵の場合、局名と再生中の曲情報を保持している。
				if radio_enable {
					infoupdate(0, mpv_infovalue)
				} else {
					infoupdate(0, errmessage[SPACE16])
				}
			case display_info_only_doubleheight_clock:
				// 高さ倍
				oled.Clear()
			case display_info_date:
				// 日付
				// 実際の表示は時刻表示の際に更新する

			default:
				// 天気予報
				infoupdate(0, info_forecast())
				if forecastinfo_enable == false {
					// 天気予報の取得に失敗している場合はそれ以上表示しないで再取得を試みる
					display_info = display_info_default
					go setup_forecast(FORECASTLOCATION)
				}
			}
		},
		irremote.Ir_C: func() {
			if state_cdx == state_AUX {
				state_event[state_AUX].btn_select_click.do_handler()
			} else {
				state_event[state_NORMAL_MODE].btn_select_press.do_handler()
			}
		},
		irremote.Ir_Center: func() {
			state_event[state_cdx].btn_mode_click.do_handler()
		},
		irremote.Ir_Center | irremote.Ir_Holdflag: func() {
			state_event[state_cdx].btn_mode_press.do_handler()
		},
		irremote.Ir_N: func() {
			state_event[state_cdx].btn_prior_click.do_handler()
		},
		irremote.Ir_N | irremote.Ir_Holdflag: func() {
			irrepeat_on = true
			irrepeat_time = time.Now()
			state_event[state_cdx].btn_prior_repeat.do_handler()
		},
		irremote.Ir_S: func() {
			state_event[state_cdx].btn_next_click.do_handler()
		},
		irremote.Ir_S | irremote.Ir_Holdflag: func() {
			irrepeat_on = true
			irrepeat_time = time.Now()
			state_event[state_cdx].btn_next_repeat.do_handler()
		},
		irremote.Ir_NW:                        inc_volume,
		irremote.Ir_NW | irremote.Ir_Holdflag: inc_volume,
		irremote.Ir_SW:                        dec_volume,
		irremote.Ir_SW | irremote.Ir_Holdflag: dec_volume,
		irremote.Ir_Power: func() {
			state_event[state_cdx].btn_select_click.do_handler()
		},
	}
)
