package main

import (
	"bufio"
	"fmt"
	"github.com/davecheney/i2c"
	"github.com/stianeikeland/go-rpio/v4"
	"local.packages/aqm1602y"
	"local.packages/irremote"
	"local.packages/mpvctl"
	"local.packages/netradio"
	"local.packages/rotaryencoder"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	stationlist     string = "/usr/local/share/mpvradio/playlists/radio.m3u"
	MPV_SOCKET_PATH string = "/run/user/1001/mpvsocket"
	VERSIONMESSAGE  string = "Radio Ver 1.32"
)

const (
	state_normal_mode    int = iota // radio on/off
	state_aux                       // bt spealer
	state_alarm_hour_set            // 時刻調整中
	state_alarm_min_set             //
	statelength
)

type eventhandler struct {
	cb   func() bool
	dflt func()
}

func _true() bool {
	return true
}
func _false() bool {
	return false
}
func _blank() {
}

func (m *eventhandler) do_handler() {
	if m.cb() == true {
		m.dflt()
	}
}

type stateEventhandlers struct {
	re_cw  eventhandler
	re_ccw eventhandler

	btn_select_click eventhandler
	btn_select_press eventhandler

	btn_mode_click   eventhandler
	btn_mode_press   eventhandler
	btn_mode_release eventhandler

	btn_next_click   eventhandler
	btn_next_repeat  eventhandler
	btn_next_release eventhandler

	btn_prior_click   eventhandler
	btn_prior_repeat  eventhandler
	btn_prior_release eventhandler
}

type ButtonCode int

const (
	btn_station_none ButtonCode = iota
	btn_station_next
	btn_station_prior
	btn_station_mode
	btn_station_select
	btn_station_re_a
	btn_station_re_b
	btn_station_repeat  = 0x8
	btn_station_press   = 0x10
	btn_station_release = 0x20

	btn_station_repeat_end = 0x80
	btn_system_shutdown    = 0x81

	btn_press_width      int = 30
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
	IR_NOT_OPEN
)

var (
	oled             aqm1602y.AQM1602Y
	mu               sync.Mutex
	stlist           []*netradio.StationInfo
	stlen            int
	colon            uint8 = 0
	pos              int   = 0
	radio_enable     bool  = false
	volume           int8  = mpvctl.Volume_max/2
	display_colon          = []uint8{' ', ':'}
	display_sleep          = []uint8{' ', ' ', 'S'}
	display_buff     []byte
	display_buff_pos int16 = 0

	display_volume           bool = false
	display_volume_time      time.Time
	display_volume_time_span time.Duration = 700 * 1000 * 1000
	weekday	= []string{"Sun","Mon","Tue","Wed","Thu","Fri","Sat"}
	clock_mode   uint8     = clock_mode_normal
	alarm_time   time.Time = time.Date(2024, time.July, 4, 4, 50, 0, 0, time.UTC)
	tuneoff_time time.Time = time.Unix(0, 0).UTC()
	errmessage             = []string{"HUP             ",
		"mpv conn error. ",
		"mpv fault.      ",
		"                ",
		"tuning error.   ",
		"rpio can't open.",
		"socket not open.",
		"BT Speaker mode ",
		"Ir not open.    "}
	btnscan         = []rpio.Pin{26, 5, 6, 22, 17, 27}
	state_cdx   int = state_normal_mode
	state_event     = [statelength]stateEventhandlers{
		// normal mode (radio)
		{eventhandler{cb: _true, dflt: inc_volume},
			eventhandler{cb: _true, dflt: dec_volume},
			eventhandler{cb: _true, dflt: toggle_radio},
			eventhandler{cb: _false, dflt: _blank},

			eventhandler{cb: _false, dflt: _blank},
			eventhandler{cb: func() bool { state_cdx = state_alarm_hour_set; return false }, dflt: _blank},
			eventhandler{cb: func() bool { state_cdx = state_alarm_hour_set; return false }, dflt: _blank},

			eventhandler{cb: _true, dflt: next_tune},
			eventhandler{cb: _true, dflt: next_station_repeat},
			eventhandler{cb: _true, dflt: tune},

			eventhandler{cb: _true, dflt: prior_tune},
			eventhandler{cb: _true, dflt: prior_station_repeat},
			eventhandler{cb: _true, dflt: tune}},
		// aux (bluetooth speaker)
		{eventhandler{cb: _true, dflt: inc_volume},
			eventhandler{cb: _true, dflt: dec_volume},
			eventhandler{cb: _true, dflt: toggle_radio},
			eventhandler{cb: _false, dflt: _blank},

			eventhandler{cb: _false, dflt: _blank},
			eventhandler{cb: _false, dflt: _blank},
			eventhandler{cb: _false, dflt: _blank},

			eventhandler{cb: _false, dflt: _blank}, // 選局ボタンを抑止
			eventhandler{cb: _false, dflt: _blank},
			eventhandler{cb: _false, dflt: _blank},

			eventhandler{cb: _false, dflt: _blank}, // 選局ボタンを抑止
			eventhandler{cb: _false, dflt: _blank},
			eventhandler{cb: _false, dflt: _blank}},
		// set hour
		{eventhandler{cb: _true, dflt: inc_volume},
			eventhandler{cb: _true, dflt: dec_volume},
			eventhandler{cb: _true, dflt: toggle_radio},
			eventhandler{cb: _false, dflt: _blank},

			eventhandler{cb: _true, dflt: func() { state_cdx = state_alarm_min_set }}, // アラーム分の設定へ遷移
			eventhandler{cb: _true, dflt: func() { state_cdx = state_alarm_min_set }},
			eventhandler{cb: _false, dflt: _blank},

			eventhandler{cb: func() bool { alarm_time_inc(); return false }, dflt: _blank},
			eventhandler{cb: func() bool { alarm_time_inc(); return true }, dflt: showclock},
			eventhandler{cb: _false, dflt: _blank},

			eventhandler{cb: func() bool { alarm_time_dec(); return false }, dflt: _blank},
			eventhandler{cb: func() bool { alarm_time_dec(); return true }, dflt: showclock},
			eventhandler{cb: _false, dflt: _blank}},
		// set min
		{eventhandler{cb: _true, dflt: inc_volume},
			eventhandler{cb: _true, dflt: dec_volume},
			eventhandler{cb: _true, dflt: toggle_radio},
			eventhandler{cb: _false, dflt: _blank},

			eventhandler{cb: _true, dflt: func() { state_cdx = state_normal_mode }},
			eventhandler{cb: _true, dflt: func() { state_cdx = state_normal_mode }},
			eventhandler{cb: _false, dflt: _blank},

			eventhandler{cb: func() bool { alarm_time_inc(); return false }, dflt: _blank},
			eventhandler{cb: func() bool { alarm_time_inc(); return true }, dflt: showclock},
			eventhandler{cb: _false, dflt: _blank},

			eventhandler{cb: func() bool { alarm_time_dec(); return false }, dflt: _blank},
			eventhandler{cb: func() bool { alarm_time_dec(); return true }, dflt: showclock},
			eventhandler{cb: _false, dflt: _blank}},
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
				// UTF-8 対応で rune　で数える
				stmp.Name = string([]rune(name + "                ")[:16])
				stlist = append(stlist, stmp)
			}
		}
	}
	return len(stlist)
}

func infoupdate(line uint8, m string) {
	mu.Lock()
	defer mu.Unlock()

	t := []byte(m)
	l := oled.UTF8toOLED(&t)
	display_buff_pos = 0
	if l >= 17 {
		display_buff = append(t[:l], append([]byte("  "), t[:l]...)...)
		oled.PrintWithPos(0, line, display_buff[:17])
	} else {
		s := append(t[:l], []byte("                ")...)
		display_buff = s[:16]
		oled.PrintWithPos(0, line, display_buff)
	}
}

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

func tune() {
	var (
		station_url, s string
		err            error = nil
	)
	radio_enable = false
	infoupdate(0, stlist[pos].Name)

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
	mpvctl.Setvol(volume)

	s = fmt.Sprintf("{\"command\": [\"loadfile\",\"%s\"]}\x0a", station_url)
	mpvctl.Send(s)
	rpio.Pin(23).High() // AF amp enable
	radio_enable = true
}

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

func showclock() {
	mu.Lock()
	defer mu.Unlock()
	var s, s0, dt string
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
	if time.Since(display_volume_time) >= display_volume_time_span {
		display_volume = false
	}
	n := time.Now()
	if display_volume {
		s = fmt.Sprintf("volume:%2d  %2d%c%02d", volume,
			n.Hour(), display_colon[colon], n.Minute())
	} else {
		s = fmt.Sprintf("%s %c   %2d%c%02d", s0,
			display_sleep[clock_mode&2],
			n.Hour(), display_colon[colon], n.Minute())
		dt = fmt.Sprintf("%04d-%02d-%02d (%s)",
			n.Year(), n.Month(), n.Day(), weekday[n.Weekday()])
	}
	oled.PrintWithPos(0, 1, []byte(s))

	// １行目の表示
	// 文字列があふれる場合はスクロールする
	// display_buff = mes + "  " + mes であることを前提としている
	if radio_enable {
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
	} else {
		oled.PrintWithPos(0, 0, []byte(dt))
	}
}

func show_volume() {
	mu.Lock()
	defer mu.Unlock()
	display_volume_time = time.Now()
	display_volume = true

	n := time.Now()
	s := fmt.Sprintf("volume:%2d  %2d%c%02d", volume,
		n.Hour(), display_colon[colon], n.Minute())
	oled.PrintWithPos(0, 1, []byte(s))
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

func toggle_radio() {
	if radio_enable {
		mpvctl.Stop()
	} else {
		tune()
	}
}

func next_tune() {
	if radio_enable == true {
		if pos < stlen-1 {
			pos++
		}
	}
	tune()
}

func next_station_repeat() {
	if pos < stlen-1 {
		pos++
		infoupdate(0, stlist[pos].Name)
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
		infoupdate(0, stlist[pos].Name)
	}
}

// mpvからの応答を選別するフィルタ
func cb_mpvrecv(ms mpvctl.MpvIRC) (string, bool) {
	//~ fmt.Printf("%#v\n",ms)
	if radio_enable {
		if ms.Event == "property-change" {
			if ms.Name == "metadata/by-key/icy-title" {
				return ms.Data, true
			}
		}
	}
	return "", false
}

func main() {
	if err := rpio.Open(); err != nil {
		infoupdate(0, errmessage[ERROR_RPIO_NOT_OPEN])
		infoupdate(1, errmessage[ERROR_HUP])
		log.Fatal(err)
	}
	defer rpio.Close()
	for _, sn := range btnscan {
		sn.Input()
		sn.PullUp()
	}
	rpio.Pin(23).Output() // AF amp 制御用
	rpio.Pin(23).PullUp()
	rpio.Pin(23).Low() // AF amp disable

	// OLED or LCD
	i2c, err := i2c.New(0x3c, 1)
	if err != nil {
		log.Fatal(err)
	}
	defer i2c.Close()
	oled = aqm1602y.New(i2c)
	oled.Configure()
	oled.PrintWithPos(0, 0, []byte(VERSIONMESSAGE))

	// rotaryencoder
	var rencoder rotaryencoder.RotaryEncoder
	rencoder = rotaryencoder.New(btnscan[btn_station_re_a-1],
		btnscan[btn_station_re_b-1],
		//~ func() {fmt.Println("cw", rencoder.GetCounter())},
		func() {},
		//~ func() {fmt.Println("ccw", rencoder.GetCounter())})
		func() {})
	rencoder.SetSamplingTime(4)

	// Ir
	if err := irremote.Open(); err != nil {
		infoupdate(0, errmessage[IR_NOT_OPEN])
		infoupdate(1, errmessage[ERROR_HUP])
		log.Fatal(err)
	}
	defer irremote.Close()
	irch := make(chan int32)
	go irremote.Read(irch)

	// mpv
	if err := mpvctl.Init(MPV_SOCKET_PATH); err != nil {
		infoupdate(0, errmessage[ERROR_MPV_FAULT])
		infoupdate(1, errmessage[ERROR_HUP])
		log.Fatal(err)
	}

	mpvctl.Cb_connect_stop = func() bool {
		infoupdate(0, errmessage[SPACE16])
		rpio.Pin(23).Low() // AF amp disable
		radio_enable = false
		return false
	}

	// シグナルハンドラ
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGQUIT,
		syscall.SIGHUP, syscall.SIGINT) // syscall.SIGUSR1

	stlen = setup_station_list()
	go netradio.Radiko_setup(stlist)

	if err := mpvctl.Open(MPV_SOCKET_PATH); err != nil {
		infoupdate(0, errmessage[ERROR_MPV_CONN])
		infoupdate(1, errmessage[ERROR_HUP])
		log.Fatal(err) // time out
	}

	mpvret := make(chan string)
	go mpvctl.Recv(mpvret, cb_mpvrecv)
	mpvctl.Setvol(volume)
	s := "{ \"command\": [\"observe_property_string\", 1, \"metadata/by-key/icy-title\"] }"
	mpvctl.Send(s)

	colonblink := time.NewTicker(500 * time.Millisecond)

	btncode := make(chan ButtonCode)
	go btninput(btncode)
	rencode := make(chan rotaryencoder.REvector)
	go rencoder.DetectLoop(rencode)

	// radioからaux(BT Speaker mode)への遷移
	state_event[state_normal_mode].btn_select_press.cb = func() bool {
		if radio_enable {
			mpvctl.Stop()
		}
		rpio.Pin(23).High() // AF amp enable
		infoupdate(0, errmessage[BT_SPEAKER])
		state_cdx = state_aux
		return false
	}

	// alarm, sleep 切り替え
	state_event[state_normal_mode].btn_mode_click.cb = func() bool {
		clock_mode++
		clock_mode &= 3
		if (clock_mode & clock_mode_sleep) != 0 {
			// スリープ時刻の設定を行う
			tuneoff_time = time.Now().Add(30 * time.Minute)
		}
		return false
	}

	// bt speaker modeからradioへの遷移
	state_event[state_aux].btn_select_click.cb = func() bool {
		// ここにペアリング先の再生を止める処理を置く
		rpio.Pin(23).Low() // AF amp disable
		infoupdate(0, errmessage[SPACE16])
		state_cdx = state_normal_mode
		return false
	}

	for {
		select {
		default:
			time.Sleep(10 * time.Millisecond)
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

		case value := <-irch:
			switch value {
			default:

			case irremote.Ir_C:
				if state_cdx == state_aux {
					state_event[state_aux].btn_select_click.do_handler()
				} else {
					state_event[state_normal_mode].btn_select_press.do_handler()
				}
			
			case irremote.Ir_Center:
				state_event[state_cdx].btn_mode_click.do_handler()
			
			case irremote.Ir_Center | irremote.Ir_Holdflag:
				state_event[state_cdx].btn_mode_press.do_handler()

			case irremote.Ir_N:
				state_event[state_cdx].btn_prior_click.do_handler()

			case irremote.Ir_N | irremote.Ir_Holdflag:
				state_event[state_cdx].btn_prior_repeat.do_handler()

			case irremote.Ir_S:
				state_event[state_cdx].btn_next_click.do_handler()

			case irremote.Ir_S | irremote.Ir_Holdflag:
				state_event[state_cdx].btn_next_repeat.do_handler()

			case irremote.Ir_NW:
				inc_volume()

			case irremote.Ir_NW | irremote.Ir_Holdflag:
				inc_volume()

			case irremote.Ir_SW:
				dec_volume()

			case irremote.Ir_SW | irremote.Ir_Holdflag:
				dec_volume()

			case irremote.Ir_Power:
				state_event[state_cdx].btn_select_click.do_handler()
			}

		case <-signals:
			mpvctl.Close()
			if err = mpvctl.Mpvkill(); err != nil {
				log.Println(err)
			}
			if err = os.Remove(MPV_SOCKET_PATH); err != nil {
				log.Println(err)
			}
			rpio.Pin(23).Low() // AF amp disable
			rpio.Close()
			irremote.Close()
			oled.DisplayOff()
			close(signals)
			os.Exit(0)

		case title := <-mpvret:
			// mpv の応答でフィルタで処理された文字列をここで処理する
			stmp := stlist[pos].Name
			if title != "" {
				stmp = stmp + "  " + title
			}
			infoupdate(0, stmp)

		case <-colonblink.C:
			colon ^= 1
			showclock()

		case r := <-rencode:
			switch r {
			default:

			case rotaryencoder.Forward:
				state_event[state_cdx].re_cw.do_handler()
			case rotaryencoder.Backward:
				state_event[state_cdx].re_ccw.do_handler()
			}

		case r := <-btncode:
			switch r {
			default:

			case (btn_system_shutdown | btn_station_repeat):
				stmp := "shutdown now    "
				infoupdate(0, stmp)
				rpio.Pin(23).Low()
				time.Sleep(700 * time.Millisecond)
				cmd := exec.Command("/usr/bin/sudo", "/usr/sbin/poweroff")
				cmd.Run()

			case btn_station_next:
				state_event[state_cdx].btn_next_click.do_handler()
			case btn_station_next | btn_station_repeat:
				state_event[state_cdx].btn_next_repeat.do_handler()
			case btn_station_next | btn_station_release:
				state_event[state_cdx].btn_next_release.do_handler()

			case btn_station_prior:
				state_event[state_cdx].btn_prior_click.do_handler()
			case btn_station_prior | btn_station_repeat:
				state_event[state_cdx].btn_prior_repeat.do_handler()
			case btn_station_prior | btn_station_release:
				state_event[state_cdx].btn_prior_release.do_handler()

			case btn_station_select:
				state_event[state_cdx].btn_select_click.do_handler()
			case btn_station_select | btn_station_press:
				state_event[state_cdx].btn_select_press.do_handler()

			case btn_station_mode:
				state_event[state_cdx].btn_mode_click.do_handler()
			case btn_station_mode | btn_station_press:
				state_event[state_cdx].btn_mode_press.do_handler()
			case btn_station_mode | btn_station_release:
				state_event[state_cdx].btn_mode_release.do_handler()

			}
		}
	}
}
