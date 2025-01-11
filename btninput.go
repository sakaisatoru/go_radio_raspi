package main

import (
	"github.com/stianeikeland/go-rpio/v4"
	"time"
)

func btninput(code chan<- ButtonCode) {
	hold := 0
	btn_h := btn_station_none

	for {
		time.Sleep(10 * time.Millisecond)
		switch btn_h {
		case 0:
			for i, sn := range btnscan[:btn_station_select] {
				// 押されているボタンがあれば、そのコードを保存する
				if sn.Read() == rpio.Low {
					btn_h = ButtonCode(i + 1)
					hold = 0
					break
				}
			}

		// もし過去になにか押されていたら、現在それがどうなっているか調べる
		default:
			for i, sn := range btnscan[:btn_station_select] {
				if btn_h == ButtonCode(i+1) {
					if sn.Read() == rpio.Low {
						// 引き続き押されている
						hold++
						if hold > btn_press_long_width {
							if btn_h == btn_station_mode {
								// mode と selectの同時押しの特殊処理
								if btnscan[btn_station_select-1].Read() == rpio.Low {
									btn_h = btn_system_shutdown
								}
							}
							// リピート入力
							// 表示が追いつかないのでリピート幅を調整すること
							hold--
							time.Sleep(150 * time.Millisecond)
							code <- (btn_h | btn_station_repeat)
						}
					} else {
						if hold >= btn_press_long_width {
							// リピート入力の終わり
							code <- (btn_h | btn_station_release)
						} else if hold > btn_press_width {
							// ワンショット入力
							code <- (btn_h | btn_station_press)
						} else if hold > 0 {
							code <- btn_h
						}
						btn_h = 0
						hold = 0
					}
					break
				}
			}
		}
	}
}
