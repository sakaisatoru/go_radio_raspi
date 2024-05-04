package main

import (
	"fmt"
	"os"
	"os/exec"
	"bufio"
	"log"
	"strings"
	"net"
	"time"
	"sync"
	"math"
	"encoding/json"
	//~ "github.com/mattn/go-tty"
	"github.com/davecheney/i2c"
	"github.com/stianeikeland/go-rpio/v4"
	"local.packages/aqm1602y"
)

const (
	stationlist string = "radio.m3u"
	MPV_SOCKET_PATH string = "/run/user/1000/mpvsocket"
	MPVOPTION1     string = "--idle"
	MPVOPTION2     string = "--input-ipc-server="+MPV_SOCKET_PATH
	MPVOPTION3     string = "--no-video"
	MPVOPTION4     string = "--no-cache"
	MPVOPTION5     string = "--stream-buffer-size=256KiB"
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

type mpvIRC struct {
    Data       	interface{}	 `json:"data"`
	Request_id  *int	 `json:"request_id"`
    Err 		string	 `json:"error"`
    Event		string	 `json:"event"`
}

var (
	mpv	net.Conn
	oled aqm1602y.AQM1602Y
	mu sync.Mutex
	stlist []*StationInfo
	colon uint8
	pos int
	readbuf = make([]byte,1024)
	mpvprocess *exec.Cmd
	volume int
	re_table = []int8{0,1,-1,0,-1,0,0,1,1,0,0,-1,0,-1,1,0}
	re_count int = 0
	display_colon = []uint8{' ',':'}
	display_sleep = []uint8{' ',' ','S'}
	clock_mode uint8
	alarm_time time.Time
	tuneoff_time time.Time
	alarm_set_mode bool
	alarm_set_pos int
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
		if n < 1024 {
			break
		}
	}
}

func mpv_playing_now() bool {
	var res []mpvIRC
	rv := false
	//~ mpv.Write([]byte("{\"command\": [\"get_property\",\"filename\"]}\x0a"))
	mpv.Write([]byte("{\"command\": [\"get_property\",\"playlist\"]}\x0a"))
	for {
		n, err := mpv.Read(readbuf)
		if err != nil {
			mes := "mpv conn error."
			infoupdate(0, &mes)
			mes = "HUP"
			infoupdate(1, &mes)
			log.Fatal(err)	
		}

exit_this:
		for _, elem := range strings.Split(string(readbuf[:n]),"\n") {
			if err := json.Unmarshal([]byte("[ "+elem+" ]"), &res); err != nil {
				continue
			}
			for _, r := range res {
				if r.Err == "property unavailable" {
					continue
				}
				if r.Err == "success" && r.Request_id != nil && r.Data != nil {
					//~ fmt.Println(r.Data)
					rv = true
					break exit_this
				}
			}
		}
		
		if n < 1024 {
			break
		}
	}
	return rv
}

func mpv_setvol(vol int) int {
	var v int
	if vol < 1 {
		v = 1
	} else {
		v = int(math.Log(float64(vol))*(100/4.6)*-1+100)
	}
	s := fmt.Sprintf("{\"command\": [\"set_property\",\"volume\",%d]}\x0a",v)
	mpv_send(s)
	return v
}

func infoupdate(line uint8, mes *string) {
	mu.Lock()
	defer mu.Unlock()

	oled.PrintWithPos(0, line, []byte(string(*mes)))
}

func btninput(code chan<- ButtonCode) {
	err := rpio.Open()
	if err != nil {
		mes := "rpio not open."
		infoupdate(0, &mes)
		mes = "HUP"
		infoupdate(1, &mes)
		log.Fatal(err)
	}
	defer rpio.Close()
	btnscan := []rpio.Pin{26, 5, 22, 6, 17, 27}
	for _, sn := range(btnscan) {
		sn.Input()
		sn.PullUp()
	}
	hold := 0
	btn_h := btn_station_none
	
	for {
		time.Sleep(10*time.Millisecond)
		// ロータリーエンコーダ
		retmp := (uint8(btnscan[4].Read())<<1) | uint8(btnscan[5].Read())
		re_count = (re_count << 2) + int(retmp)
		n := re_table[re_count & 15]
		if n != 0 {
			volume += int(n)
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
					//~ if rpio.ReadPin(sn) == rpio.Low {
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
						//~ if rpio.ReadPin(sn) == rpio.Low {
						if sn.Read() == rpio.Low {
							// 引き続き押されている
							hold++
							if hold > btn_press_long_width {
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
			stmp := "Tuning Error    "
			infoupdate(0, &stmp)
		}
	} else {
		s := fmt.Sprintf("{\"command\": [\"loadfile\",\"%s\"]}\x0a", stlist[pos].url)
		mpv_send(s)
	}
}

func radio_stop() {
	mpv_send("{\"command\": [\"stop\"]}\x0a")
	stmp := "                "
	infoupdate(0, &stmp)
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
	oled.PrintWithPos(0,1,[]byte(s))
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
												MPVOPTION5)
	err = mpvprocess.Start()
	if err != nil {
		stmp := "mpv fault."
		infoupdate(0, &stmp)
		stmp = "HUP"
		infoupdate(1, &stmp)
		log.Fatal(err)
	}

	setup_station_list()
	stlen := len(stlist)

	for i := 0; ;i++ {
		mpv, err = net.Dial("unix", MPV_SOCKET_PATH);
		if err == nil {
			defer mpv.Close()
			break
		}
		time.Sleep(200*time.Millisecond)
		if i > 30 {
			mes := "mpv conn error."
			infoupdate(0, &mes)
			mes = "HUP"
			infoupdate(1, &mes)
			log.Fatal(err)	// time out
		}
	}

	colonblink := time.NewTicker(500*time.Millisecond)
	 
	pos = 0
	volume = 40
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
	go btninput(btncode)
	
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
						
					case (btn_station_select):
						if mpv_playing_now() == true {
							//~ fmt.Println("true")
							radio_stop()
						} else {
							//~ fmt.Println("false")
							tune()
						}
					
					case (btn_station_select|btn_station_repeat):
						//~ select_btn_repeat_count++
						//~ if select_btn_repeat_count > 3 {
							//~ radio_stop()
						//~ }
						
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
	
	// shutdown this program
	err = mpvprocess.Process.Kill()
	if err != nil {
		log.Println(err)
	}
	oled.DisplayOff()
}
