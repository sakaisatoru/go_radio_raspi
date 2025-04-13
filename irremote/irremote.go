package irremote

import (
	"golang.org/x/sys/unix"
	"syscall"
	"unsafe"
)

type Input_event struct {
	Tv    syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

const (
	Ir_Holdflag    = 0x10000
	Ir_Releaseflag = 0x20000

	KEY_STOP       = 128   // 0x10d8
	KEY_A          = 30    // 0x10f8
	KEY_B          = 48    // 0x1078
	KEY_C          = 46    // 0x1058
	KEY_UP         = 103   // 0x10a0
	KEY_DOWN       = 108   // 0x1000
	KEY_LEFT       = 105   // 0x1010
	KEY_RIGHT      = 106   // 0x1080
	KEY_SELECT     = 0x161 // 0x1020
	KEY_VOLUMEUP   = 115   // 0x10b1      # UP - LEFT
	KEY_VOLUMEDOWN = 114   // 0x1011    # DOWN - LEFT
	KEY_PAGEUP     = 104   // 0x1021        # UP - RIGHT
	KEY_PAGEDOWN   = 109   // 0x1081      # DOWN - RIGHT

	key_press   = 1
	key_repeat  = 2
	key_release = 0
)

var (
	fd int
)

func Open() error {
	var err error
	fd, err = syscall.Open("/dev/input/event0", syscall.O_RDONLY, 0)
	return err
}

func Close() {
	syscall.Close(fd)
}

func Read(ch chan<- int32) {
	var (
		buf    = make([]byte, 24)
		ev     *Input_event
		repeat bool = false
	)

	for {
		_, err := syscall.Read(fd, buf)
		if err != nil {
			break
		}
		ev = (*Input_event)(unsafe.Pointer(&buf[0]))
		if ev.Type == unix.EV_KEY {
			switch ev.Value {
			case key_press:
				ch <- int32(ev.Code)
			case key_repeat:
				repeat = true
				ch <- int32(ev.Code) | Ir_Holdflag
			case key_release:
				if repeat {
					repeat = false
					ch <- int32(ev.Code) | Ir_Releaseflag
				}
			}
		}
	}
}
