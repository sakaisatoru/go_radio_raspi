package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"bufio"
	"log"
	"strings"
	"net"
	"time"
	"sync"
	"github.com/davecheney/i2c"
	"github.com/stianeikeland/go-rpio/v4"
	"local.packages/aqm1602y"
	"local.packages/netradio"
	"local.packages/mpvctl"
)

const (
	stationlist string = "/usr/local/share/mpvradio/playlists/radio.m3u"
	MPV_SOCKET_PATH string = "/run/mpvsocket"
	RADIO_SOCKET_PATH string = "/run/mpvradio"
	VERSIONMESSAGE string = "Radio Ver 1.21"
)

const (
	state_normal_mode int = iota	// radio on/off
	state_ext_mode					// alarm, sleep, aux 切り替え
	state_aux						// bt spealer
	state_alarm_hour_set			// 時刻調整中
	state_alarm_min_set				//
	statelength
)

type stateEventhandlersDefault struct {
	__re_cw				func()
	__re_ccw				func()
	
	__btn_select_click		func()
	__btn_select_press		func()
	
	__btn_mode_click		func()
	__btn_mode_press		func()
	__btn_mode_release		func()

	__btn_next_click		func()
	__btn_next_repeat		func()
	__btn_next_release		func()

	__btn_prior_click		func()
	__btn_prior_repeat		func()
	__btn_prior_release	func()
	
} 

type stateEventhandlers struct {
	cb_re_cw				func()
	cb_re_ccw				func()
	
	cb_btn_select_click		func()
	cb_btn_select_press		func()
	
	cb_btn_mode_click		func()
	cb_btn_mode_press		func()
	cb_btn_mode_release		func()

	cb_btn_next_click		func()
	cb_btn_next_repeat		func()
	cb_btn_next_release		func()

	cb_btn_prior_click		func()
	cb_btn_prior_repeat		func()
	cb_btn_prior_release	func()
	
} 

type ButtonCode int
const (
	btn_station_none ButtonCode = iota
	btn_station_next 
	btn_station_prior
	btn_station_mode
	btn_station_select
	btn_station_re_forward
	btn_station_re_backward
	btn_station_repeat = 0x8
	btn_station_press  = 0x10
	btn_station_release = 0x20

	btn_station_repeat_end = 0x80
	btn_system_shutdown = 0x81
	
	btn_press_width int = 30
	btn_press_long_width int = 90
)

const (
	clock_mode_normal uint8 = iota
	clock_mode_alarm
	clock_mode_sleep
	clock_mode_sleep_alarm
)

const (
	ERROR_HUP = iota
	ERROR_MPV_CONN
	ERROR_MPV_FAULT
	SPACE16
	ERROR_TUNING
	ERROR_RPIO_NOT_OPEN
	ERROR_SOCKET_NOT_OPEN
	BT_SPEAKER
)

var (
	mpv	net.Conn
	oled aqm1602y.AQM1602Y
	mu sync.Mutex
	stlist []*netradio.StationInfo
	stlen int
	colon uint8 = 0
	pos int = 0
	radio_enable bool = false
	volume int8 = 60
	display_colon = []uint8{' ',':'}
	display_sleep = []uint8{' ',' ','S'}
	display_buff string = ""
	display_buff_pos int16 = 0
	clock_mode uint8 = clock_mode_normal
	alarm_time time.Time = time.Date(2024, time.July, 4, 4, 50, 0, 0, time.UTC)
	tuneoff_time time.Time = time.Unix(0, 0).UTC()
	errmessage = []string{"HUP             ",
						"mpv conn error. ",
						"mpv fault.      ",
						"                ",
						"tuning error.   ",
						"rpio can't open.",
						"socket not open.",
						"BT Speaker mode "}
	btnscan = []rpio.Pin{26, 5, 6, 22, 17, 27}
	state_cdx int = state_normal_mode
	state_event = [statelength]stateEventhandlers {
		{inc_volume, dec_volume, toggle_radio, func() {},func() {},func() {},func() {},	next_tune, next_station_repeat, tune, prior_tune, prior_station_repeat, tune},
		{inc_volume, dec_volume, toggle_radio, func() {},func() {},func() {},func() {},	next_tune, next_station_repeat, tune, prior_tune, prior_station_repeat, tune},
		{inc_volume, dec_volume, toggle_radio, func() {},func() {},func() {},func() {},	next_tune, next_station_repeat, tune, prior_tune, prior_station_repeat, tune},
		{inc_volume, dec_volume, toggle_radio, func() {},func() {},func() {},func() {},	next_tune, next_station_repeat, tune, prior_tune, prior_station_repeat, tune},
		{inc_volume, dec_volume, toggle_radio, func() {},func() {},func() {},func() {},	next_tune, next_station_repeat, tune, prior_tune, prior_station_repeat, tune},
	} 
)

func setup_station_list() int {
	file, err := os.Open(stationlist)
	if err != nil {
		log.Fatal(err)
	} 
	defer file.Close()

	scanner := bufio.NewScanner(file)
	f := false
	s := ""
	name := ""
	for scanner.Scan() {
		s = scanner.Text()
		if strings.Contains(s, "#EXTINF:") == true {
			f = true
			_, name, _ = strings.Cut(s, "/")
			name = strings.Trim(name, " ")
			continue
		}
		if f {
			if len(s) != 0 {
				f = false
				stmp := new(netradio.StationInfo)
				stmp.Url = s
				stmp.Name = string(name+"                ")[:16]
				stlist = append(stlist, stmp)
			}
		}
	}
	return len(stlist)
}

func infoupdate(line uint8, mes *string) {
	mu.Lock()
	defer mu.Unlock()
	display_buff_pos = 0
	if len(*mes) >= 17 {
		if line == 0 {
			display_buff = *mes + "  " + *mes
		}
		oled.PrintWithPos(0, line, []byte(*mes)[:17])
	} else {
		if line == 0 {
			display_buff = *mes
		}
		oled.PrintWithPos(0, line, []byte(*mes))
	}
}

func btninput(code chan<- ButtonCode) {
	rpio.Pin(23).Output()	// AF amp 制御用
	rpio.Pin(23).PullUp()
	rpio.Pin(23).Low()		// AF amp disable	
	
	hold := 0
	btn_h := btn_station_none
	
	var n int
	
	for {
		time.Sleep(10*time.Millisecond)
		// ロータリーエンコーダ
		// エッジを検出することで直前の相からの遷移方向を判断する。
		// 両方検出した場合はノイズとして扱う
		b4 := btnscan[5].Read()
		b3 := btnscan[4].Read()
		n = 0
		switch (b4 << 1 | b3) {
			case 0:
				if btnscan[5].EdgeDetected() {
					n++
				}
				if btnscan[4].EdgeDetected() {
					n--
				}
				btnscan[4].Detect(rpio.RiseEdge)
				btnscan[5].Detect(rpio.RiseEdge)
			case 1:
				if btnscan[4].EdgeDetected() {
					n++
				}
				if btnscan[5].EdgeDetected() {
					n--
				}
				btnscan[5].Detect(rpio.RiseEdge)
				btnscan[4].Detect(rpio.FallEdge)
			case 3:
				if btnscan[5].EdgeDetected() {
					n++
				}
				if btnscan[4].EdgeDetected() {
					n--
				}
				btnscan[4].Detect(rpio.FallEdge)
				btnscan[5].Detect(rpio.FallEdge)
			case 2:
				if btnscan[4].EdgeDetected() {
					n++
				}
				if btnscan[5].EdgeDetected() {
					n--
				}
				btnscan[5].Detect(rpio.FallEdge)
				btnscan[4].Detect(rpio.RiseEdge)
		}
		
		switch n {
			case 1:
				code <- btn_station_re_forward
			case -1:
				code <- btn_station_re_backward
			default:
				// ノイズとして無視する
		}

		switch btn_h {
			case 0:
				for i, sn := range(btnscan[:btn_station_select]) {
					// 押されているボタンがあれば、そのコードを保存する
					if sn.Read() == rpio.Low {
						btn_h = ButtonCode(i+1)
						hold = 0
						break
					}
				}

			// もし過去になにか押されていたら、現在それがどうなっているか調べる
			default:
				for i, sn := range(btnscan[:btn_station_select]) {
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
								time.Sleep(150*time.Millisecond)
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

func tune() {
	var (
		station_url string
		err error = nil
	)
	
	infoupdate(0, &stlist[pos].Name)
	
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
			return 
		}
	} else {
		station_url = stlist[pos].Url
	}

	s := fmt.Sprintf("{\"command\": [\"loadfile\",\"%s\"]}\x0a", station_url)
	mpvctl.Send(s)
	rpio.Pin(23).High()		// AF amp enable
	radio_enable = true
}

func alarm_time_inc() {
	if state_cdx == state_alarm_hour_set {
		alarm_time = alarm_time.Add(1*time.Hour)
	} else {
		alarm_time = alarm_time.Add(1*time.Minute)
	}
}

func alarm_time_dec() {
	if state_cdx == state_alarm_min_set {
		alarm_time = alarm_time.Add(59*time.Minute)
		// 時間が進んでしまうのでhourも補正する
	}
	alarm_time = alarm_time.Add(23*time.Hour)
}

func showclock() {
	mu.Lock()
	defer mu.Unlock()
	var s0 string
	// alarm
	if clock_mode & 1 == 1 {
		if (state_cdx == state_alarm_hour_set||state_cdx == state_alarm_min_set) && colon == 1 {
			if state_cdx == state_alarm_hour_set {
				s0 = fmt.Sprintf("A  :%02d", alarm_time.Minute()) // blink hour
			} else {
				s0 = fmt.Sprintf("A%02d:  ", alarm_time.Hour()) // blink min
			} 
		} else {
			s0 = fmt.Sprintf("A%02d:%02d", alarm_time.Hour(),
											alarm_time.Minute())
		}
	} else {
		s0 = "      "
	}
	n := time.Now()
	s := fmt.Sprintf("%s %c   %02d%c%02d", s0, 
							display_sleep[clock_mode & 2], 
							n.Hour(), display_colon[colon], n.Minute())
	oled.PrintWithPos(0, 1, []byte(s))
	
	// １行目の表示
	// 文字列があふれる場合はスクロールする
	// display_buff = *mes + "  " + *mes であることを前提としている
	display_buff_len := len(display_buff)
	if display_buff_len <= 16 {
		oled.PrintWithPos(0, 0, []byte(display_buff))
	} else {
		oled.PrintWithPos(0, 0, []byte(display_buff)[display_buff_pos:display_buff_pos+17])
		display_buff_pos++
		if display_buff_pos >= int16((display_buff_len/2)+1) {
			display_buff_pos = 0
		} 
	}
}

func recv_title(socket net.Listener) {
	var stmp string
	buf := make([]byte, mpvctl.IRCbuffsize)
	for {
		n := func() int {
			conn, err := socket.Accept()
			if err != nil {
				return 0
			}
			defer conn.Close()
			n := 0
			for {
				n, err = conn.Read(buf)
				if err != nil {
					return 0
				}
				if n < mpvctl.IRCbuffsize {
					break
				}
			}
			conn.Write([]byte("OK\n"))
			return n
		}()
		if radio_enable == true {
			stmp = stlist[pos].Name + "  " + string(buf[:n])
		} else {
			stmp = string(buf[:n]) + "  "
		}
		infoupdate(0, &stmp)
	}
}

func inc_volume() {
	if radio_enable {
		volume++
		if volume > 99 { 
			volume = 99 
		}
		mpvctl.Setvol(volume)
	}
}
	
func dec_volume() {
	if radio_enable {
		volume--
		if volume < 0 {
			volume = 0
		}
		mpvctl.Setvol(volume)
	}
}

func toggle_radio() {
	if radio_enable {
		mpvctl.Stop()
	} else {
		tune()
	}
}

func next_tune() {
	if radio_enable == true {
		if pos < stlen -1 {
			pos++
		}
	}
	tune()
}

func next_station_repeat() {
	if pos < stlen -1 {
		pos++
		infoupdate(0, &stlist[pos].Name)
	}	
}

func prior_tune() {
	if radio_enable == true {
		if pos > 0 {
			pos--
		}
	}
	tune()
}

func prior_station_repeat() {
	if pos > 0 {
		pos--
		infoupdate(0, &stlist[pos].Name)
	}
}

func main() {
	if err := rpio.Open();err != nil {
		infoupdate(0, &errmessage[ERROR_RPIO_NOT_OPEN])
		infoupdate(1, &errmessage[ERROR_HUP])
		log.Fatal(err)
	}
	defer rpio.Close()
	for _, sn := range(btnscan) {
		sn.Input()
		sn.PullUp()
	}
	rpio.Pin(23).Output()	// AF amp 制御用
	rpio.Pin(23).PullUp()
	rpio.Pin(23).Low()		// AF amp disable	
	
	// OLED or LCD
	i2c, err := i2c.New(0x3c, 1)
	if err != nil {
		log.Fatal(err)
	}
	defer i2c.Close()
	oled = aqm1602y.New(i2c)
	oled.Configure()
	oled.PrintWithPos(0, 0, []byte(VERSIONMESSAGE))

	radiosocket, err := net.Listen("unix", RADIO_SOCKET_PATH)
	if err != nil {
		infoupdate(0, &errmessage[ERROR_SOCKET_NOT_OPEN])
		infoupdate(1, &errmessage[ERROR_HUP])
		log.Fatal(err)
	}

	if err := mpvctl.Init(MPV_SOCKET_PATH);err != nil {
		infoupdate(0, &errmessage[ERROR_MPV_FAULT])
		infoupdate(1, &errmessage[ERROR_HUP])
		log.Fatal(err)
	}
	
	mpvctl.Cb_connect_stop = func() bool {
		infoupdate(0, &errmessage[SPACE16])
		rpio.Pin(23).Low()		// AF amp disable
		radio_enable = false
		return false
	}
	// シグナルハンドラ
	go func() {
		// shutdown this program
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM, syscall.SIGQUIT, 
					syscall.SIGHUP, syscall.SIGINT) // syscall.SIGUSR1
		
		for {
			switch <-signals {
				case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGINT:
					mpvctl.Close()
					if err = mpvctl.Mpvkill();err != nil {
						log.Println(err)
					}
					radiosocket.Close()
					if err = os.Remove(MPV_SOCKET_PATH);err != nil {
						log.Println(err)
					}
					if err = os.Remove(RADIO_SOCKET_PATH);err != nil {
						log.Println(err)
					}
					rpio.Pin(23).Low()		// AF amp disable
					oled.DisplayOff()
					close(signals)
					os.Exit(0)
			}
		}
	}()
	
	stlen = setup_station_list()
	go netradio.Radiko_setup(stlist)
	
	if mpvctl.Open(MPV_SOCKET_PATH) != nil {
		infoupdate(0, &errmessage[ERROR_MPV_CONN])
		infoupdate(1, &errmessage[ERROR_HUP])
		log.Fatal(err)	// time out
	}
	mpvctl.Setvol(volume)

	colonblink := time.NewTicker(500*time.Millisecond)

	btncode := make(chan ButtonCode)
	go btninput(btncode)
	go recv_title(radiosocket)
	
	
	// radio
	state_event[state_normal_mode].cb_btn_select_press		 = func() {
		if radio_enable {
			mpvctl.Stop()
		}
		rpio.Pin(23).High()		// AF amp enable
		infoupdate(0, &errmessage[BT_SPEAKER])
		state_cdx = state_aux
	}
	state_event[state_normal_mode].cb_btn_mode_click		 = func() {
		if clock_mode == 0 {
			clock_mode = 1
		}
		state_cdx = state_ext_mode	// モード遷移
	}

	// alarm, sleep 切り替え
	state_event[state_ext_mode].cb_btn_mode_click		 	 = func() {
		clock_mode++
		clock_mode &= 3
		if (clock_mode & clock_mode_sleep) != 0 {
			// スリープ時刻の設定を行う
			tuneoff_time = time.Now().Add(30*time.Minute)
		}
	}
	state_event[state_ext_mode].cb_btn_mode_release = state_event[state_ext_mode].cb_btn_mode_click

	state_event[state_ext_mode].cb_btn_mode_press		 	 = func() {
		state_cdx = state_alarm_hour_set	// アラーム時の設定へ遷移
	}
	state_event[state_ext_mode].cb_btn_next_click			 = func() {
		state_cdx = state_normal_mode
		state_event[state_cdx].cb_btn_next_click()
	}
	state_event[state_ext_mode].cb_btn_next_repeat 			 = func() {
		state_cdx = state_normal_mode
		state_event[state_cdx].cb_btn_next_repeat()
	} 
	state_event[state_ext_mode].cb_btn_next_release  		 = func() {
		state_cdx = state_normal_mode
		state_event[state_cdx].cb_btn_next_release()
	}
	state_event[state_ext_mode].cb_btn_prior_click			 = func() {
		state_cdx = state_normal_mode
		state_event[state_cdx].cb_btn_prior_click()
	}
	state_event[state_ext_mode].cb_btn_prior_repeat 		 = func() {
		state_cdx = state_normal_mode
		state_event[state_cdx].cb_btn_prior_repeat()
	} 
	state_event[state_ext_mode].cb_btn_prior_release  		 = func() {
		state_cdx = state_normal_mode
		state_event[state_cdx].cb_btn_prior_release()
	}

	// bt spealer mode
	state_event[state_aux].cb_btn_next_click				 = func() {
		// ここにbtを止める処理を置く
		state_event[state_aux].cb_btn_select_click()
		tune()
	}
	state_event[state_aux].cb_btn_prior_click = state_event[state_aux].cb_btn_next_click				
	state_event[state_aux].cb_btn_select_click 				 = func() {
		// bt_stop()
		rpio.Pin(23).Low()		// AF amp disable
		infoupdate(0, &errmessage[SPACE16])
		state_cdx = state_normal_mode
	}

	// set alarm
	state_event[state_alarm_hour_set].cb_btn_mode_click		 = func() {
		state_cdx = state_alarm_min_set		// アラーム分の設定へ遷移
	}
	state_event[state_alarm_hour_set].cb_btn_mode_press = state_event[state_alarm_hour_set].cb_btn_mode_click
	state_event[state_alarm_hour_set].cb_btn_next_click		 = func() {
		alarm_time_inc()
	}
	state_event[state_alarm_hour_set].cb_btn_next_repeat	 = func() {
		alarm_time_inc()
		showclock() // 表示が追いつかないのでここでも更新する
	}
	state_event[state_alarm_hour_set].cb_btn_prior_click	 = func() {
		alarm_time_dec()
	}
	state_event[state_alarm_hour_set].cb_btn_prior_repeat	 = func() {
		alarm_time_dec()
		showclock() // 表示が追いつかないのでここでも更新する
	}


	state_event[state_alarm_min_set].cb_btn_mode_click		 	 = func() {
		state_cdx = state_ext_mode			// alarm,sleepの切替へ遷移
	}
	state_event[state_alarm_min_set].cb_btn_mode_press = state_event[state_alarm_min_set].cb_btn_mode_click
	state_event[state_alarm_min_set].cb_btn_next_click = state_event[state_alarm_hour_set].cb_btn_next_click
	state_event[state_alarm_min_set].cb_btn_next_repeat = state_event[state_alarm_hour_set].cb_btn_next_repeat
	state_event[state_alarm_min_set].cb_btn_prior_click = state_event[state_alarm_hour_set].cb_btn_prior_click
	state_event[state_alarm_min_set].cb_btn_prior_repeat = state_event[state_alarm_hour_set].cb_btn_prior_repeat
	
	for {
		select {
			default:
				time.Sleep(10*time.Millisecond)
				if (state_cdx != state_alarm_hour_set) && (state_cdx != state_alarm_min_set) {
					if (clock_mode & clock_mode_alarm) != 0 {
						// アラーム
						n := time.Now()
						if alarm_time.Hour() == n.Hour() &&
						   alarm_time.Minute() == n.Minute() {
							clock_mode ^= clock_mode_alarm
							tune()
						}
					}
					if (clock_mode & clock_mode_sleep) != 0 {
						// スリープ
						n := time.Now()
						if tuneoff_time.Hour() == n.Hour() &&
						   tuneoff_time.Minute() == n.Minute() {
							clock_mode ^= clock_mode_sleep
							mpvctl.Stop()
						}
					}
				}
				
			case <-colonblink.C:
				colon ^= 1
				showclock()
				
			case r := <-btncode:
				switch r {
					default:

					case (btn_system_shutdown|btn_station_repeat):
						stmp := "shutdown now    "
						infoupdate(0, &stmp)
						rpio.Pin(23).Low()
						time.Sleep(700*time.Millisecond)
						cmd := exec.Command("/usr/bin/sudo", "/usr/sbin/poweroff")
						cmd.Run()

					case btn_station_re_forward:
						state_event[state_cdx].cb_re_cw()
					case btn_station_re_backward:
						state_event[state_cdx].cb_re_ccw()

					case btn_station_next: 
						state_event[state_cdx].cb_btn_next_click()
					case btn_station_next|btn_station_repeat:
						state_event[state_cdx].cb_btn_next_repeat()
					case btn_station_next|btn_station_release:
						state_event[state_cdx].cb_btn_next_release()

					case btn_station_prior: 
						state_event[state_cdx].cb_btn_prior_click()
					case btn_station_prior|btn_station_repeat:
						state_event[state_cdx].cb_btn_prior_repeat()
					case btn_station_prior|btn_station_release:
						state_event[state_cdx].cb_btn_prior_release()
						
					case btn_station_select:
						state_event[state_cdx].cb_btn_select_click()
					case btn_station_select|btn_station_press:
						state_event[state_cdx].cb_btn_select_press()

					case btn_station_mode:
						state_event[state_cdx].cb_btn_mode_click()
					case btn_station_mode|btn_station_press:
						state_event[state_cdx].cb_btn_mode_press()
					case btn_station_mode|btn_station_release:
						state_event[state_cdx].cb_btn_mode_release()
			
				}
		}
	}
}
