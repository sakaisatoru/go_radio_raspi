package mpvctl

import (
		"fmt"
		"net"
		"time"
		"os/exec"
)

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
	IRCbuffsize int = 1024
	MPVOPTION1     string = "--idle"
	MPVOPTION2     string = "--input-ipc-server="
	MPVOPTION3     string = "--no-video"
	MPVOPTION4     string = "--no-cache"
	MPVOPTION5     string = "--stream-buffer-size=256KiB"
	MPVOPTION6	   string = "--script=/home/pi/bin/title_trigger.lua"
)

var (
	mpv net.Conn
	mpvprocess *exec.Cmd
	volconv = []int8{	 0, 1, 2, 3, 4, 4, 5, 6, 6,  7, 
						 7, 8, 8, 9, 9,10,10,11,11, 11,
						12,12,13,13,13,14,14,14,15, 15,
						16,16,16,17,17,17,18,18,18, 19,
						19,20,20,20,21,21,22,22,23, 23,
						24,24,25,25,26,26,27,27,28, 28,
						29,30,30,31,32,32,33,34,35, 35,
						36,37,38,39,40,41,42,43,45, 46,
						47,49,50,52,53,55,57,59,61, 63,
						66,68,71,74,78,81,85,90,95,100}
	readbuf = make([]byte, IRCbuffsize)
	Cb_connect_stop = func() bool { return false } 
)

func Init(socketpath string) error {
	mpvprocess = exec.Command("/usr/bin/mpv", 	MPVOPTION1, 
												MPVOPTION2+socketpath, 
												MPVOPTION3, MPVOPTION4, 
												MPVOPTION5, MPVOPTION6)
	err := mpvprocess.Start()
	return err
}

func Mpvkill() error {
	err := mpvprocess.Process.Kill()
	return err
}

func Open(socket_path string) error {
	var err error
	for i := 0; ;i++ {
		mpv, err = net.Dial("unix", socket_path)
		if err == nil {
			break
		}
		time.Sleep(200*time.Millisecond)
		if i > 60 {
			return err	// time out
		}
	}
	return nil
}

func Close() {
	mpv.Close()
}

func Send(s string) error {
	mpv.Write([]byte(s))
	for {
		n, err := mpv.Read(readbuf)
		if err != nil {
			return err
		}
		if n < IRCbuffsize {
			break
		}
	}
	return nil
}

func Setvol(vol int8) {
	if vol < 1 {
		vol = 0
	} else if vol >= 100 {
		vol = 99
	} 
	s := fmt.Sprintf("{\"command\": [\"set_property\",\"volume\",%d]}\x0a",volconv[vol])
	Send(s)
}

func Stop() {
	if Cb_connect_stop() == false {
		Send("{\"command\": [\"stop\"]}\x0a")
	}
}

