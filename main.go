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
	"github.com/mattn/go-tty"
	"golang.org/x/text/transform"
	"golang.org/x/text/encoding/japanese"
	"github.com/davecheney/i2c"
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

type StationInfo struct {
	name string
	url string
}

var (
	mpv	net.Conn
	stlist []*StationInfo
	readbuf = make([]byte,1024)
	mpvprocess *exec.Cmd
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

func mpv_setvol(vol int) {
	s := fmt.Sprintf("{\"command\": [\"set_property\",\"volume\",%d]}\x0a",vol)
	mpv_send(s)
}

func beep() {
	s := fmt.Sprintf("{\"command\": [\"loadfile\",\"%s\"]}\x0a", "/usr/local/share/mpvradio/sounds/button57.mp3")
	mpv_send(s)
}


func main() {
	// OLED or LCD
	i2c, err := i2c.New(0x3c, 1)
	if err != nil {
		log.Fatal(err)
	}
	defer i2c.Close()
	oled := aqm1602y.New(i2c)
	oled.Configure()
	oled.PrintWithPos(0, 0, []byte("test"))

	tty, err := tty.Open();
	if err != nil {
		log.Fatal(err)
	}
	defer tty.Close()
	
	mpvprocess = exec.Command("/usr/bin/mpv",MPVOPTION1, MPVOPTION2, MPVOPTION3, MPVOPTION4, MPVOPTION5)
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
		//~ time.Sleep(100*time.Millisecond)
		time.Sleep(1000*time.Millisecond)
		if i > 30 {
			// time out
			log.Fatal(err)
		}
	}
	
	
	
	pos := 0
	volume := 30
	mpv_setvol (volume)
	
	for {
		// ラズパイでのリストがshift_jisなので、端末表示用にutf-8に変換する
		stmp, n, err := transform.String(japanese.ShiftJIS.NewDecoder(), stlist[pos].name)
		if err != nil {
			log.Fatal(err)
		}
		oled.PrintWithPos(0, 0, []byte(stlist[pos].name))
		fmt.Printf("\r                                                  \r%s %d",stmp, n)
		//~ fmt.Printf("\r                                              \r%s",stlist[pos].name)
		r, err := tty.ReadRune()
		if err != nil {
			log.Fatal(err)
		}
		switch string(r) {
			case "a":
				if volume < 100 {
					volume += 10
					if volume > 100 {
						volume = 100
					}
				}
				mpv_setvol (volume)
				stmp := fmt.Sprintf("vol : %d", volume)
				oled.Clear()
				oled.PrintWithPos(0, 1, []byte(stmp))
				continue
			case "z":
				if volume > 0 {
					volume -= 10
					if volume < 0 {
						volume = 0
					}
				}
				mpv_setvol (volume)
				stmp := fmt.Sprintf("vol : %d", volume)
				oled.Clear()
				oled.PrintWithPos(0, 1, []byte(stmp))
				continue
			case "s":
				oled.Clear()
				mpv_send("{\"command\": [\"stop\"]}\x0a")
				continue
			case "n":
				if pos < stlen -1 {
					oled.Clear()
					pos++
				}
				continue
			case "b":
				if pos > 0 {
					oled.Clear()
					pos--
				}
				continue
			case " ":
				args := strings.Split(stlist[pos].url, "/")
				if args[0] == "plugin:" {
					cmd := exec.Command("/usr/local/share/mpvradio/plugins/"+args[1], args[2])
					err := cmd.Run()
					if err != nil {
						log.Fatal(err)
					}
				} else {
					fmt.Println(stlist[pos].url)
					s := fmt.Sprintf("{\"command\": [\"loadfile\",\"%s\"]}\x0a", stlist[pos].url)
					//~ fmt.Println(s)
					mpv_send(s)
				}
				continue
			case "q":
				break
		}
		break
	}
	
	// shutdown this program
	err = mpvprocess.Process.Kill()
	if err != nil {
		log.Fatal(err)
	}
	oled.DisplayOff()
}
