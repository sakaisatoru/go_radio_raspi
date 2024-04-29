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
	"github.com/mattn/go-tty"
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
	btn_station_select
	btn_station_re_forward
	btn_station_re_backward
	btn_station_repeat_end
	
	btn_station_repeat = 0x80
	
	btn_press_width int = 10
	btn_press_long_width int = 80
	
)

type StationInfo struct {
	name string
	url string
}

var (
	mpv	net.Conn
	oled aqm1602y.AQM1602Y
	linebuf1 string
	linebuf2 string
	stlist []*StationInfo
	colon uint8
	pos int
	readbuf = make([]byte,1024)
	mpvprocess *exec.Cmd
	volume int
	re_table = []int8{0,1,-1,0,-1,0,0,1,1,0,0,-1,0,-1,1,0}
	re_count int = 0
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
		if f {
			if len(s) != 0 {
				f = false
				stmp := new(StationInfo)
				stmp.url = s
				stmp.name = name
				stlist = append(stlist, stmp)
			}
		}
		if strings.Contains(s, "#EXTINF:") == true {
			f = true
			_, name, _ = strings.Cut(s, "/")
			name = strings.Trim(name, " ")
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

func mpv_setvol(vol int) int {
	var v int
	if vol < 1 {
		v = 0
	} else {
		v = int(math.Log(float64(vol))*(100/4.6)*-1+100)
	}
	s := fmt.Sprintf("{\"command\": [\"set_property\",\"volume\",%d]}\x0a",v)
	mpv_send(s)
	return v
}

func beep() {
	s := fmt.Sprintf("{\"command\": [\"loadfile\",\"%s\"]}\x0a", "/usr/local/share/mpvradio/sounds/button57.mp3")
	mpv_send(s)
}

var mu sync.Mutex
func infoupdate() {
	mu.Lock()
	defer mu.Unlock()
	
	oled.PrintWithPos(0, 0, []byte(string(linebuf1+"                ")[:15]))
	oled.PrintWithPos(0, 1, []byte(string(linebuf2+"                ")[:15]))
}

func btninput(code chan<- ButtonCode) {
	err := rpio.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer rpio.Close()
	btnscan := []rpio.Pin{26, 5, 6, 17, 27}
	for _, sn := range(btnscan) {
		sn.Input()
		sn.PullUp()
	}
	hold := 0
	btn_h := btn_station_none
	var m3 sync.Mutex
	
	for {
		time.Sleep(10*time.Millisecond)
		// ロータリーエンコーダ
		retmp := (uint8(btnscan[3].Read())<<1) | uint8(btnscan[4].Read())
		re_count = (re_count << 2) + int(retmp)
		n := re_table[re_count & 15]
		if n != 0 {
			volume += int(n)
			if volume > 100 {
				volume = 100
			} else if volume < 0 {
				volume = 0
			}
			//~ fmt.Printf("volume : %d\n", volume)
			m3.Lock()
			linebuf2 = fmt.Sprintf("volume : %d",
											mpv_setvol (volume))
			m3.Unlock()
		}

		switch btn_h {
			case 0:
				for i, sn := range(btnscan[:3]) {
					// 押されているボタンがあれば、そのコードを保存する
					if rpio.ReadPin(sn) == rpio.Low {
						btn_h = ButtonCode(i+1)
						hold = 0
						break
					}
				}

			// もし過去になにか押されていたら、現在それがどうなっているか
			// 調べる
			default:
				for i, sn := range(btnscan[:3]) {
					if btn_h == ButtonCode(i+1) {
						if rpio.ReadPin(sn) == rpio.Low {
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
							if hold == btn_press_long_width {
								code <- btn_station_repeat_end
							} else if hold > btn_press_width && hold < btn_press_long_width {
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
	args := strings.Split(stlist[pos].url, "/")
	if args[0] == "plugin:" {
		cmd := exec.Command("/usr/local/share/mpvradio/plugins/"+args[1], args[2])
		err := cmd.Run()
		if err != nil {
			linebuf1 = "Tuning Error"
		}
	} else {
		s := fmt.Sprintf("{\"command\": [\"loadfile\",\"%s\"]}\x0a", stlist[pos].url)
		mpv_send(s)
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

	tty, err := tty.Open();
	if err != nil {
		log.Fatal(err)
	}
	defer tty.Close()
	
	mpvprocess = exec.Command("/usr/bin/mpv", MPVOPTION1, MPVOPTION2, MPVOPTION3, MPVOPTION4, MPVOPTION5)
	err = mpvprocess.Start()
	if err != nil {
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
			log.Fatal(err)	// time out
		}
	}

	colonblink := time.NewTicker(500*time.Millisecond)
	
	linebuf1 = ""
	linebuf2 = ""
	pos = 0
	volume = 50
	mpv_setvol (volume)
	colon = 0
	var m2 sync.Mutex
	btncode := make(chan ButtonCode)
	go btninput(btncode)
	
	for {
		select {
			case <-colonblink.C:
				colon ^= 1
			case r := <-btncode:
				switch r {
					//~ case (btn_station_select|btn_station_repeat):
					case (btn_station_select):
						mpv_send("{\"command\": [\"stop\"]}\x0a")
						linebuf1 = "stop"
						
					case btn_station_repeat_end:
							linebuf1 = stlist[pos].name
							tune()
						
					case (btn_station_next|btn_station_repeat):
							if pos < stlen -1 {
								pos++
								linebuf1 = stlist[pos].name
							}
						
					case btn_station_next:
							if pos < stlen -1 {
								pos++
								linebuf1 = stlist[pos].name
								tune()
							}
						
					case (btn_station_prior|btn_station_repeat):
							if pos > 0 {
								pos--
								linebuf1 = stlist[pos].name
							}
						
					case btn_station_prior:
							if pos > 0 {
								pos--
								linebuf1 = stlist[pos].name
								tune()
							}
				}

			default:
				time.Sleep(100*time.Millisecond)
				cs := " "
				if colon == 1 {
					cs = ":"
				}
				m2.Lock()
				linebuf2 = fmt.Sprintf("%02d%s%02d", time.Now().Hour(),cs,time.Now().Minute())
				m2.Unlock()
				infoupdate()
		}
	}
	
	// shutdown this program
	err = mpvprocess.Process.Kill()
	if err != nil {
		log.Fatal(err)
	}
	oled.DisplayOff()
}
