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
	//~ "bytes"
	//~ "encoding/json"
	"github.com/davecheney/i2c"
	"github.com/stianeikeland/go-rpio/v4"
	"local.packages/aqm1602y"
)

const (
	stationlist string = "/usr/local/share/mpvradio/playlists/radio.m3u"
	MPV_SOCKET_PATH string = "/run/mpvsocket"
	MPVOPTION1     string = "--idle"
	MPVOPTION2     string = "--input-ipc-server="+MPV_SOCKET_PATH
	MPVOPTION3     string = "--no-video"
	MPVOPTION4     string = "--no-cache"
	MPVOPTION5     string = "--stream-buffer-size=256KiB"
	MPVOPTION6	   string = "--script=/home/pi/bin/title_trigger.lua"
	mpvIRCbuffsize int = 1024
	RADIO_SOCKET_PATH string = "/run/mpvradio"
)

type ButtonCode int
const (
	btn_station_none ButtonCode = iota
	btn_station_next 
	btn_station_prior
	btn_station_mode
	btn_station_select
	btn_station_re_forward
	btn_station_re_backward
	btn_station_repeat_end
	btn_system_shutdown
	
	btn_station_repeat = 0x80
	
	btn_press_width int = 7
	btn_press_long_width int = 80
)

const (
	clock_mode_normal uint8 = iota
	clock_mode_alarm
	clock_mode_sleep
	clock_mode_sleep_alarm
)

type StationInfo struct {
	name string
	url string
}

type MpvIRCdata struct {
	Filename	*string		`json:"filename"`
	Current		bool		`json:"current"`
	Playing		bool		`json:"playing"`
}
 
type mpvIRC struct {
    Data       	*MpvIRCdata	 `json:"data"`
	Request_id  *int	 `json:"request_id"`
    Err 		string	 `json:"error"`
    Event		string	 `json:"event"`
}

const (
	ERROR_HUP = iota
	ERROR_MPV_CONN
	ERROR_MPV_FAULT
	SPACE16
	ERROR_TUNING
	ERROR_RPIO_NOT_OPEN
	ERROR_SOCKET_NOT_OPEN
)

var (
	mpv	net.Conn
	oled aqm1602y.AQM1602Y
	mu sync.Mutex
	stlist []*StationInfo
	colon uint8
	pos int
	radio_enable bool
	readbuf = make([]byte, mpvIRCbuffsize)
	mpvprocess *exec.Cmd
	volume int8
	display_colon = []uint8{' ',':'}
	display_sleep = []uint8{' ',' ','S'}
	display_buff string
	display_buff_len int8
	display_buff_pos int8
	clock_mode uint8
	alarm_time time.Time
	tuneoff_time time.Time
	alarm_set_mode bool
	alarm_set_pos int
	errmessage = []string{"HUP             ",
						"mpv conn error. ",
						"mpv fault.      ",
						"                ",
						"tuning error.   ",
						"rpio can't open.",
						"socket not open."}
)

func setup_station_list () {
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
				stmp := new(StationInfo)
				stmp.url = s
				stmp.name = string(name+"                ")[:16]
				stlist = append(stlist, stmp)
			}
		}
	}
}

func mpv_send(s string) {
	mpv.Write([]byte(s))
	for {
		n, err := mpv.Read(readbuf)
		if err != nil {
			log.Fatal(err)
		}
		//~ fmt.Println(string(readbuf[:n]))
		if n < mpvIRCbuffsize {
			break
		}
	}
}

var (
	volconv = []int8{	0,1,2,3,4,4,5,6,6,7,7,8,8,9,9,10,10,11,11,
						11,12,12,13,13,13,14,14,14,15,15,16,16,16,17,
						17,17,18,18,18,19,19,20,20,20,21,21,22,22,23,
						23,24,24,25,25,26,26,27,27,28,28,29,30,30,31,
						32,32,33,34,35,35,36,37,38,39,40,41,42,43,45,
						46,47,49,50,52,53,55,57,59,61,63,66,68,71,74,
						78,81,85,90,95,100}
)

func mpv_setvol(vol int8) {
	if vol < 1 {
		vol = 0
	} else if vol >= 100 {
		vol = 99
	} 
	s := fmt.Sprintf("{\"command\": [\"set_property\",\"volume\",%d]}\x0a",volconv[vol])
	mpv_send(s)
}

func infoupdate(line uint8, mes *string) {
	mu.Lock()
	defer mu.Unlock()
	display_buff_len = int8(len(*mes))
	display_buff_pos = 0
	if display_buff_len >= 17 {
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
	err := rpio.Open()
	if err != nil {
		infoupdate(0, &errmessage[ERROR_RPIO_NOT_OPEN])
		infoupdate(1, &errmessage[ERROR_HUP])
		log.Fatal(err)
	}
	defer rpio.Close()
	//~ btnscan := []rpio.Pin{26, 5, 22, 6, 17, 27}
	btnscan := []rpio.Pin{26, 5, 6, 22, 17, 27}
	for _, sn := range(btnscan) {
		sn.Input()
		sn.PullUp()
	}
	
	rpio.Pin(23).Output()	// AF amp 制御用
	rpio.Pin(23).PullUp()
	rpio.Pin(23).Low()		// AF amp disable	
	
	hold := 0
	btn_h := btn_station_none
	
	re_table := []int8{0,1,-1,0,-1,0,0,1,1,0,0,-1,0,-1,1,0}
	re_count := 0

	for {
		time.Sleep(10*time.Millisecond)
		// ロータリーエンコーダ
		retmp := (uint8(btnscan[5].Read())<<1) | uint8(btnscan[4].Read())
		re_count = (re_count << 2) + int(retmp)
		n := re_table[re_count & 15]
		if n != 0 {
			volume += n
			if volume > 100 {
				volume = 100
			} else if volume < 1 {
				volume = 1
			}
			mpv_setvol (volume)
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

			// もし過去になにか押されていたら、現在それがどうなっているか
			// 調べる
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
								code <- btn_station_repeat_end
							} else if hold > btn_press_width {
								// ワンショット入力
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
	infoupdate(0, &stlist[pos].name)
	
	args := strings.Split(stlist[pos].url, "/")
	if args[0] == "plugin:" {
		cmd := exec.Command("/usr/local/share/mpvradio/plugins/"+args[1], args[2])
		err := cmd.Run()
		if err != nil {
			infoupdate(0, &errmessage[ERROR_TUNING])
			radio_enable = false
		} else {
			rpio.Pin(23).High()		// AF amp enable
			radio_enable = true
		}
	} else {
		s := fmt.Sprintf("{\"command\": [\"loadfile\",\"%s\"]}\x0a", stlist[pos].url)
		mpv_send(s)
		rpio.Pin(23).High()		// AF amp enable
		radio_enable = true
	}
}

func radio_stop() {
	mpv_send("{\"command\": [\"stop\"]}\x0a")
	infoupdate(0, &errmessage[SPACE16])
	rpio.Pin(23).Low()		// AF amp disable
	radio_enable = false
}

func alarm_time_inc () {
	if alarm_set_pos == 0 {
		// hour
		alarm_time = alarm_time.Add(1*time.Hour)
	} else {
		// minute
		alarm_time = alarm_time.Add(1*time.Minute)
	}
	alarm_time = time.Date(2009, 1, 1, alarm_time.Hour(), alarm_time.Minute(), 0, 0, time.UTC)
}

func alarm_time_dec () {
	if alarm_set_pos == 1 {
		// minute
		// 時間が進んでしまうのでhourも補正する
		alarm_time = alarm_time.Add(59*time.Minute)
	}
	// hour
	alarm_time = alarm_time.Add(23*time.Hour)
	alarm_time = time.Date(2009, 1, 1, alarm_time.Hour(), alarm_time.Minute(), 0, 0, time.UTC)
}

func showclock() {
	mu.Lock()
	defer mu.Unlock()
	var s0 string
	// alarm
	if clock_mode & 1 == 1 {
		if alarm_set_mode && colon == 1 {
			if alarm_set_pos == 0 {
				// blink hour
				s0 = fmt.Sprintf("A  :%02d", alarm_time.Minute())
			} else {
				// blink minute
				s0 = fmt.Sprintf("A%02d:  ", alarm_time.Hour())
			} 
		} else {
			s0 = fmt.Sprintf("A%02d:%02d", alarm_time.Hour(),
											alarm_time.Minute())
		}
	} else {
		s0 = "      "
	}
	
	s2 := fmt.Sprintf("%02d%c%02d", time.Now().Hour(),
									display_colon[colon],
										time.Now().Minute())
	s := fmt.Sprintf("%s %c   %s", s0, display_sleep[clock_mode & 2], s2)
	oled.PrintWithPos(0, 1, []byte(s))
	
	// １行目の表示
	// 文字列があふれる場合はスクロールする
	if display_buff_len <= 16 {
		oled.PrintWithPos(0, 0, []byte(display_buff))
	} else {
		oled.PrintWithPos(0, 0, []byte(display_buff)[display_buff_pos:display_buff_pos+17])
		display_buff_pos++
		if display_buff_pos >= int8(display_buff_len + 2) {
			display_buff_pos = 0
		} 
	}
}


func recv_title(socket net.Listener) {
	var stmp string
	buf := make([]byte, 1024)
	for {
		n := func () int {
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
				if n < 1024 {
					break
				}
			}
			conn.Write([]byte("OK\n"))
			return n
		}()
		if radio_enable == true {
			stmp = stlist[pos].name + "  " + string(buf[:n])
		} else {
			stmp = string(buf[:n]) + "  "
		}
		infoupdate(0, &stmp)
	}
}

func main() {
	// OLED or LCD
	i2c, err := i2c.New(0x3c, 1)
	if err != nil {
		log.Fatal(err)
	}
	defer i2c.Close()
	oled = aqm1602y.New(i2c)
	oled.Configure()
	oled.PrintWithPos(0, 0, []byte("radio"))

	mpvprocess = exec.Command("/usr/bin/mpv", 	MPVOPTION1, MPVOPTION2, 
												MPVOPTION3, MPVOPTION4, 
												MPVOPTION5, MPVOPTION6)
	
	radiosocket, err := net.Listen("unix", RADIO_SOCKET_PATH)
	if err != nil {
		infoupdate(0, &errmessage[ERROR_SOCKET_NOT_OPEN])
		infoupdate(1, &errmessage[ERROR_HUP])
		log.Fatal(err)
	}
	defer radiosocket.Close()

	err = mpvprocess.Start()
	if err != nil {
		infoupdate(0, &errmessage[ERROR_MPV_FAULT])
		infoupdate(1, &errmessage[ERROR_HUP])
		log.Fatal(err)
	}
	
	// シグナルハンドラ
	go func() {
		// shutdown this program
		signals := make(chan os.Signal, 1)
		//~ signal.Notify(signals, syscall.SIGUSR1, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGINT)
		signal.Notify(signals, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGINT)
		
		for {
			s := <-signals
			switch s {
				//~ case syscall.SIGUSR1:
					//~ stmp := stlist[pos].name + "  " + mpv_get_title ()
					//~ infoupdate(0, &stmp)
				case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGINT:
					err = mpvprocess.Process.Kill()
					if err != nil {
						log.Println(err)
					}
					err = os.Remove(MPV_SOCKET_PATH)
					if err != nil {
						log.Println(err)
					}
					err = os.Remove(RADIO_SOCKET_PATH)
					if err != nil {
						log.Println(err)
					}
					rpio.Pin(23).Low()		// AF amp disable
					oled.DisplayOff()
					close(signals)
					os.Exit(0)
			}
		}
	}()
	
	setup_station_list()
	stlen := len(stlist)

	for i := 0; ;i++ {
		mpv, err = net.Dial("unix", MPV_SOCKET_PATH);
		if err == nil {
			defer mpv.Close()
			break
		}
		time.Sleep(200*time.Millisecond)
		if i > 60 {
			infoupdate(0, &errmessage[ERROR_MPV_CONN])
			infoupdate(1, &errmessage[ERROR_HUP])
			log.Fatal(err)	// time out
		}
	}

	colonblink := time.NewTicker(500*time.Millisecond)
	
	radio_enable = false
	pos = 0
	volume = 60
	mpv_setvol (volume)
	colon = 0
	clock_mode = clock_mode_normal
	
	select_btn_repeat_count := 0

	mode_btn_repeat_count := 0
	alarm_set_mode = false
	alarm_set_pos = 0
	alarm_time = time.Unix(0, 0).UTC()
	tuneoff_time = time.Unix(0, 0).UTC()
	btncode := make(chan ButtonCode)
	display_buff = ""
	display_buff_pos = 0
	
	go btninput(btncode)
	go recv_title(radiosocket)
	
	for {
		select {
			default:
				time.Sleep(10*time.Millisecond)
				if alarm_set_mode == false {
					if (clock_mode & clock_mode_alarm) != 0 {
						// アラーム
						if alarm_time.Hour() == time.Now().Hour() &&
						   alarm_time.Minute() == time.Now().Minute() {
							clock_mode ^= clock_mode_alarm
							tune()
						}
					}
					if (clock_mode & clock_mode_sleep) != 0 {
						// スリープ
						if tuneoff_time.Hour() == time.Now().Hour() &&
						   tuneoff_time.Minute() == time.Now().Minute() {
							clock_mode ^= clock_mode_sleep
							radio_stop()
						}
					}
				}
				
			case <-colonblink.C:
				colon ^= 1
				showclock()
				
			case r := <-btncode:
				switch r {
					case btn_system_shutdown|btn_station_repeat:
						stmp := "shutdown now    "
						infoupdate(0, &stmp)
						rpio.Pin(23).Low()
						time.Sleep(700*time.Millisecond)
						cmd := exec.Command("/usr/bin/sudo", "/usr/sbin/poweroff")
						cmd.Run()
						
					case btn_station_mode:
						if alarm_set_mode {
							// アラーム設定時は変更桁の遷移を行う
							alarm_set_pos++
							if alarm_set_pos >= 2 {
								alarm_set_mode = false
							}
						} else {
							// 通常時はアラーム、スリープのオンオフを行う
							clock_mode++
							clock_mode &= 3
							if (clock_mode & clock_mode_sleep) != 0 {
								// スリープ時刻の設定を行う
								tuneoff_time = time.Now().Add(30*time.Minute)
							}
						}
						
					case (btn_station_mode|btn_station_repeat):
						// alarm set
						mode_btn_repeat_count++
						if mode_btn_repeat_count > 3 {
							// アラーム時刻の設定へ
							alarm_set_mode = true
							alarm_set_pos = 0
						}
						
					case (btn_station_select|btn_station_repeat):
						select_btn_repeat_count++
						fallthrough
						
					case btn_station_select:
						if radio_enable {
							radio_stop()
						} else {
							tune()
						}
						
					case btn_station_repeat_end:
						if alarm_set_mode == false {
							if select_btn_repeat_count == 0 && mode_btn_repeat_count == 0 { 
								tune()
							}
						}
						select_btn_repeat_count = 0
						mode_btn_repeat_count = 0
						
					case (btn_station_next|btn_station_repeat):
						if alarm_set_mode {
							// アラーム設定時は時刻設定を行う
							// リピート時の表示が追いつかないのでここでも表示する
							alarm_time_inc ()
							showclock()
						} else {
							if pos < stlen -1 {
								pos++
								infoupdate(0, &stlist[pos].name)
							}
						}
						
					case btn_station_next:
						if alarm_set_mode {
							// アラーム設定時は時刻設定を行う
							alarm_time_inc ()
						} else {
							if pos < stlen -1 {
								pos++
								tune()
							}
						}

					case (btn_station_prior|btn_station_repeat):
						if alarm_set_mode {
							// アラーム設定時は時刻設定を行う
							// リピート時の表示が追いつかないのでここでも表示する
							alarm_time_dec ()
							showclock()
						} else {
							if pos > 0 {
								pos--
								infoupdate(0, &stlist[pos].name)
							}
						}

					case btn_station_prior:
						if alarm_set_mode {
							// アラーム設定時は時刻設定を行う
							alarm_time_dec ()
						} else {
							if pos > 0 {
								pos--
								tune()
							}
						}
				}
		}
	}
}
