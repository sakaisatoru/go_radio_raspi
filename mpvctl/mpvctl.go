package mpvctl

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

type MpvIRC struct {
	Data       string `json:"data"`
	Name       string `json:"name"`
	Request_id int    `json:"request_id"`
	Err        string `json:"error"`
	Event      string `json:"event"`
}

func (ms *MpvIRC) clear() {
	ms.Data = ""
	ms.Name = ""
	ms.Request_id = 0
	ms.Err = ""
	ms.Event = ""
}

const (
	IRCbuffsize int    = 1024
	MPVOPTION1  string = "--idle"
	MPVOPTION2  string = "--input-ipc-server="
	MPVOPTION3  string = "--no-video"
	MPVOPTION4  string = "--no-cache"
	MPVOPTION5  string = "--stream-buffer-size=256KiB"
)

var (
	mpv        net.Conn
	mpvprocess *exec.Cmd
	volconv	*[]int8
	voldefault = []int8{ 0, 8,12,16, 22,28,34,41,
						48,56,61,65, 69}
						//~ 48,56,61,65, 69,72,75,78,
						//~ 81,83,85,87, 89,91,94, 100}
	Volume_min      int8
	Volume_max      int8
	readbuf              = make([]byte, IRCbuffsize)
	Cb_connect_stop      = func() bool { return false }
	socketpath      string
)

func SetVoltable(t *[]int8) {
	if t == nil {
		volconv = &voldefault
	} else {
		volconv = t
	}
	Volume_min = 0
	Volume_max = int8(len(*volconv) - 1)
}

func Init(s string) error {
	SetVoltable(nil)
	socketpath = s
	mpvprocess = exec.Command("/usr/bin/mpv", MPVOPTION1,
		MPVOPTION2+socketpath,
		MPVOPTION3, MPVOPTION4,
		MPVOPTION5)
	err := mpvprocess.Start()
	return err
}

func Mpvkill() error {
	err := mpvprocess.Process.Kill()
	e2 := os.Remove(socketpath)
	if err != nil {
		return err
	}
	return e2
}

func Open() error {
	var err error
	for i := 0; ; i++ {
		mpv, err = net.Dial("unix", socketpath)
		if err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
		if i > 60 {
			return err // time out
		}
	}
	return nil
}

func Close() {
	mpv.Close()
}

func Send(s string) error {
	_, err := mpv.Write([]byte(s))
	return err
}

func Recv(ch chan<- string, cb func(MpvIRC) (string, bool)) {
	var ms MpvIRC

	for {
		n, err := mpv.Read(readbuf)
		if err == nil {
			if n < IRCbuffsize {
				for _, s := range strings.Split(string(readbuf[:n]), "\n") {
					if len(s) > 0 {
						ms.clear() // 中身を消さないとフィールド単位で持ち越される場合がある
						err := json.Unmarshal([]byte(s), &ms)
						if err == nil {
							s, ok := cb(ms)
							if ok {
								ch <- s
							}
						}
					}
				}
			}
		}
	}
}

func Setvol(vol int8) error {
	if vol < Volume_min {
		vol = Volume_min
	} else if vol > Volume_max {
		vol = Volume_max
	}
	s := fmt.Sprintf("{\"command\": [\"set_property\",\"volume\",%d]}\x0a", (*volconv)[vol])
	return Send(s)
}

func Stop() error {
	if Cb_connect_stop() == false {
		return Send("{\"command\": [\"stop\"]}\x0a")
	}
	return nil
}

func Loadfile(s string) error {
	return Send(fmt.Sprintf("{\"command\": [\"loadfile\",\"%s\"]}\x0a", s))
}

