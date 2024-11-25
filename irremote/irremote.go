package irremote

import (
	"fmt"
	"syscall"
	"unsafe"
	"time"
	"golang.org/x/sys/unix"
	)

type Input_event struct {
	Tv syscall.Timeval
	Type uint16
	Code uint16
	Value int32
}

const (
		Ir_power = 0x10d8
		Ir_A 	= 0x10f8
		Ir_B 	= 0x1078
		Ir_C 	= 0x1058
		Ir_N	= 0x10A0
		Ir_NE	= 0x1021
		Ir_E	= 0x1080
		Ir_SE	= 0x1081
		Ir_S	= 0x1000
		Ir_SW	= 0x1011
		Ir_W	= 0x1010
		Ir_NW	= 0x10B1
		Ir_Center = 0x1020
		Ir_Holdflag = 0x10000
)

var (
		fd int
)

func Open() (int,error) {
	var err error
	fd, err = syscall.Open("/dev/input/event0", syscall.O_RDONLY, 0)
	return fd, err
}

func Close() {
	syscall.Close(fd)
}

func Read(ch chan<- int32) {
	var( 
		buf = make([]byte, 24) 
		ev	*Input_event
		t_span time.Duration = 110*1e6	// 110ms
	)
	hold_span := 8
	hold_count := 0
	old := Input_event {
			Value:0,}
	t_start := time.Now()
	hold := false
	release := false
	
	for {
		_, err := syscall.Read(fd, buf)
		if err != nil {
			//~ fmt.Println(err)
			break
		}
		ev = (*Input_event)(unsafe.Pointer(&buf[0]))
		if ev.Type == unix.EV_MSC {
			if ev.Value != old.Value || time.Since(t_start) > t_span {
				release = true
				hold = false
				hold_count = 0
			}
			
			t_start = time.Now()
			old = *ev
			
			if hold {
				ch <- (ev.Value |Ir_Holdflag)
				//~ fmt.Printf("hold : value = %08X  code = %04X\n", ev.Value, ev.Code)
				continue
			}
			
			if release {
				ch <- ev.Value
				//~ fmt.Printf("hold : value = %08X  code = %04X\n", ev.Value, ev.Code)
				release = false
				continue
			}
			
			if release == false {
				hold_count++
				if hold_count > hold_span {
					hold = true
				}
			}
		} 
	}

}

func main() {
	fd,err := syscall.Open("/dev/input/event0", syscall.O_RDONLY, 0)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer syscall.Close(fd)
	
	var( 
		buf = make([]byte, 24) 
		ev	*Input_event
		t_span time.Duration = 110*1e6	// 110ms
	)
	hold_span := 8
	hold_count := 0
	old := Input_event {
			Value:0,}
	t_start := time.Now()
	hold := false
	release := false
	
	for {
		_, err := syscall.Read(fd, buf)
		if err != nil {
			fmt.Println(err)
			break
		}
		ev = (*Input_event)(unsafe.Pointer(&buf[0]))
		if ev.Type == unix.EV_MSC {
			if ev.Value != old.Value || time.Since(t_start) > t_span {
				release = true
				hold = false
				hold_count = 0
			}
			
			t_start = time.Now()
			old = *ev
			
			if hold {
				fmt.Printf("hold : value = %08X  code = %04X\n", ev.Value, ev.Code)
				continue
			}
			
			if release {
				fmt.Printf("hold : value = %08X  code = %04X\n", ev.Value, ev.Code)
				release = false
				continue
			}
			
			if release == false {
				hold_count++
				if hold_count > hold_span {
					hold = true
				}
			}
		} 
	}
}
