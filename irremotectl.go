package main

import (
	"local.packages/irremote"
)

var (
	irfunc = map[int32]func(){
		irremote.KEY_A: func() {
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
		irremote.KEY_SELECT: func() {
			state_event[state_cdx].btn_mode_click.do_handler()
		},
		irremote.KEY_SELECT | irremote.Ir_Releaseflag: func() {
			state_event[state_cdx].btn_mode_release.do_handler()
		},
		irremote.KEY_UP: func() {
			state_event[state_cdx].btn_prior_click.do_handler()
		},
		irremote.KEY_UP | irremote.Ir_Holdflag: func() {
			state_event[state_cdx].btn_prior_repeat.do_handler()
		},
		irremote.KEY_UP | irremote.Ir_Releaseflag: func() {
			state_event[state_cdx].btn_prior_release.do_handler()
		},
		irremote.KEY_DOWN: func() {
			state_event[state_cdx].btn_next_click.do_handler()
		},
		irremote.KEY_DOWN | irremote.Ir_Holdflag: func() {
			state_event[state_cdx].btn_next_repeat.do_handler()
		},
		irremote.KEY_DOWN | irremote.Ir_Releaseflag: func() {
			state_event[state_cdx].btn_next_release.do_handler()
		},
		irremote.KEY_VOLUMEUP:                          inc_volume,
		irremote.KEY_VOLUMEUP | irremote.Ir_Holdflag:   inc_volume,
		irremote.KEY_VOLUMEDOWN:                        dec_volume,
		irremote.KEY_VOLUMEDOWN | irremote.Ir_Holdflag: dec_volume,
		irremote.KEY_STOP: func() {
			state_event[state_cdx].btn_select_click.do_handler()
		},
	}
)
