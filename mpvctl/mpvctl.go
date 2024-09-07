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
	volconv = []int8{	 0, 2, 4, 5, 6,  7, 8, 9,10,11,
						12,13,13,14,15,	16,16,17,18,18,
						19,20,21,22,23,	24,25,26,27,28,
						29,30,32,33,35,	36,38,40,42,45,
						47,50,53,57,61,	66,71,78,85,100}
	Volume_min int8 = 0
	Volume_max int8 = int8(len(volconv) - 1)
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
	if vol < Volume_min {
		vol = Volume_min
	} else if vol > Volume_max {
		vol = Volume_max
	} 
	s := fmt.Sprintf("{\"command\": [\"set_property\",\"volume\",%d]}\x0a",volconv[vol])
	Send(s)
}

func Stop() {
	if Cb_connect_stop() == false {
		Send("{\"command\": [\"stop\"]}\x0a")
	}
}

