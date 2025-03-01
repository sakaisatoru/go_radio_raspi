package irremote

import (
	"golang.org/x/sys/unix"
	"syscall"
	"time"
	"unsafe"
)

type Input_event struct {
	Tv    syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

const (
	Ir_Power    = 0x10d8
	Ir_A        = 0x10f8
	Ir_B        = 0x1078
	Ir_C        = 0x1058
	Ir_N        = 0x10A0
	Ir_NE       = 0x1021
	Ir_E        = 0x1080
	Ir_SE       = 0x1081
	Ir_S        = 0x1000
	Ir_SW       = 0x1011
	Ir_W        = 0x1010
	Ir_NW       = 0x10B1
	Ir_Center   = 0x1020
	Ir_Holdflag = 0x10000

	T_span time.Duration = 110 * 1e6 // 110ms
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
		buf = make([]byte, 24)
		ev  *Input_event
	)
	hold_span := 8
	hold_count := 0
	old := Input_event{
		Value: 0}
	t_start := time.Now()
	hold := false
	release := false

	for {
		_, err := syscall.Read(fd, buf)
		if err != nil {
			break
		}
		ev = (*Input_event)(unsafe.Pointer(&buf[0]))
		if ev.Type == unix.EV_MSC {
			if ev.Value != old.Value || time.Since(t_start) > T_span {
				release = true
				hold = false
				hold_count = 0
			}

			t_start = time.Now()
			old = *ev

			if hold {
				ch <- (ev.Value | Ir_Holdflag)
				continue
			}

			if release {
				ch <- ev.Value
				release = false
				continue
			}

			if release == false {
				hold_count++
				if hold_count > hold_span {
					hold = true
					ch <- (ev.Value | Ir_Holdflag)
					continue
				}
			}
		}
	}

}
