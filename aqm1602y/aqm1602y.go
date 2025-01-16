package aqm1602y

import (
	"github.com/davecheney/i2c"
	"sync"
	"time"
)

const (
	i2c_SLAVE = 0x0000
)

type Config struct {
}

type AQM1602Y struct {
	bus		i2c.I2C
	Config	Config
	mu		sync.Mutex
	display_buff     []byte
	display_buff_pos int
	display_buff_len int
}

var (
// ラテン１補助 (Latin-1-Supplement) の１部分 C2A0 - C2BF
	t_C2A0 = [...]byte{0x3F, 0xe9, 0xe4, 0xe5, 0x3F, 0xe6, 0x7c, 0x3F,
						0xf1, 0x3F, 0x61, 0xfb, 0x3F, 0x3F, 0x3F, 0xff,
						0xdf, 0x3F, 0x32, 0x33, 0xf4, 0x75, 0x3F, 0xa5,
						0x3F, 0x31, 0x30, 0xfc, 0xf6, 0xf5, 0x3F, 0x9f}

// ラテン１補助 (Latin-1-Supplement) の１部分 C380 - C3BF
	t_C380 = [...]byte{  0x41, 0x41, 0x8f, 0xea, 0x8e, 0x41, 0x92, 0x80,
						 0x45, 0x90, 0x45, 0x45, 0x49, 0x49, 0x49, 0x49,
						 
						 0x44, 0x4e, 0x4f, 0x4f, 0x4f, 0xec, 0x4f, 0xf7,
						 0xee, 0x55, 0x55, 0x55, 0x9a, 0x59, 0x3F, 0x3F,
						 
						 0x85, 0xe0, 0x83, 0xeb, 0x84, 0x61, 0x91, 0x87,
						 0x8a, 0x82, 0x88, 0x89, 0x8d, 0xe1, 0x8c, 0x8b,
						 
						 0x64, 0x9b, 0x95, 0xe2, 0x93, 0xed, 0x94, 0xf8,
						 0xee, 0x97, 0xe3, 0x96, 0x81, 0x79, 0x3F, 0x79}
// ギリシア文字
	t_CE90 = [...]byte{  0x3f, 0x41, 0x42, 0x09, 0x15, 0x45, 0x5a, 0x48,
						 0x16, 0x49, 0x4b, 0x17, 0x4d, 0x4e, 0x18, 0x4f,

						 0x19, 0x50, 0x3f, 0x1a, 0x54, 0x59, 0xef, 0x58, 
						 0x1d, 0x1e, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f}
		
// 半角・全角形 (Halfwidth and Fullwidth Forms) の１部分 半角カナ EFBDA0 - EFBDBF
	t_EFBDA0 = [...]byte{0x20, 0xA1, 0xA2, 0xA3, 0xA4, 0xA5, 0xA6, 0xA7,
						 0xA8, 0xA9, 0xAA, 0xAB, 0xAC, 0xAD, 0xAE, 0xAF,
						 0xB0, 0xB1, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6, 0xB7,
						 0xB8, 0xB9, 0xBA, 0xBB, 0xBC, 0xBD, 0xBE, 0xBF}

// 半角・全角形 (Halfwidth and Fullwidth Forms) の１部分 半角カナ EFBE80 - EFBE9F
	t_EFBE80 = [...]byte{0xC0, 0xC1, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7,
						 0xC8, 0xC9, 0xCA, 0xCB, 0xCC, 0xCD, 0xCE, 0xCF,
						 0xD0, 0xD1, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7,
						 0xD8, 0xD9, 0xDA, 0xDB, 0xDC, 0xDD, 0xDE, 0xDF}
)

func (d *AQM1602Y) UTF8toOLED(s *[]byte) int {
	var	rv	[]byte
	rv = *s
	l := len(rv)
	pos := 0
	pass_count := 0
	for i, v := range rv {
		if pass_count > 0 {
			pass_count--
			continue
		}
		if v >= 0x20 && v <= 0x7f {
			rv[pos] = v
			pos++
			continue
		} 
		
		switch {
			case v == 0xc2:
				if l <= i + 1 {
					v = 0x3f
				} else if rv[i+1] >= 0xa0 && rv[i+1] <= 0xbf {
					pass_count = 1
					v = t_C2A0[rv[i+1]-0xa0]
				} else {
					pass_count = 1
					v = 0x3f	// '?'
				}
				
			case v == 0xc3:
				if l <= i + 1 {
					v = 0x3f
				} else if rv[i+1] >= 0x80 && rv[i+1] <= 0xbf {
					pass_count = 1
					v = t_C380[rv[i+1]-0x80]
				} else {
					pass_count = 1
					v = 0x3f	// '?'
				}

			case v == 0xce:
				if l <= i + 1 {
					v = 0x3f
				} else if rv[i+1] >= 0x90 && rv[i+1] <= 0xaf {
					pass_count = 1
					v = t_CE90[rv[i+1]-0x90]
				} else {
					pass_count = 1
					v = 0x3f
				}
				
			case v >= 0xc4 && v <= 0xdf:
				pass_count = 1
				v = 0x3f
				
			case v == 0xef:
				if l <= i + 2 {
					v = 0x3f
				} else if rv[i+1] == 0xbd {
					if rv[i+2] >= 0xa0 && rv[i+2] <= 0xbf {
						v = t_EFBDA0[rv[i+2]-0xa0]
					} else {
						v = 0x3f
					}
					pass_count = 2
				} else if rv[i+1] == 0xbe {
					if rv[i+2] >= 0x80 && rv[i+2] <= 0x9f {
						v = t_EFBE80[rv[i+2]-0x80]
					} else {
						v = 0x3f
					}
					pass_count = 2
				}
			
			case v >= 0xe0 && v <= 0xee:
				pass_count = 2
				v = 0x3f
				
			case v >= 0xf0 && v <= 0xf4:
				pass_count = 3
				v = 0x3f
		}
		rv[pos] = v
		pos++
	}
	return pos
}


func New(bus *i2c.I2C) AQM1602Y {
	return AQM1602Y {
		bus:		*bus,
		display_buff_pos: 0,
	}
}

func (d *AQM1602Y) Configure() {
	d.mu.Lock()
	defer d.mu.Unlock()
	// OLED 初期化ルーチン
	time.Sleep(100 * time.Millisecond)                 // power on 後の推奨待ち時間
	d.bus.Write([]byte{0x00, 0x01}) // clear
	time.Sleep(20 * time.Millisecond)
	d.bus.Write([]byte{0x00, 0x02}) // home
	time.Sleep(2 * time.Millisecond)
	d.bus.Write([]byte{0x00, 0x0c}) // display on
}

func (d *AQM1602Y) ConfigureWithSettings(config Config) {
}

func (d *AQM1602Y) Init() {
}

func (d *AQM1602Y) SetDoubleHeight() {
	d.bus.Write([]byte{0x00, 0x2c})
	time.Sleep(20 * time.Millisecond)
}

func (d *AQM1602Y) SetNormal() {
	d.bus.Write([]byte{0x00, 0x28})
	time.Sleep(20 * time.Millisecond)
}

func (d *AQM1602Y) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bus.Write([]byte{0x00, 0x01}) // display off
	time.Sleep(20 * time.Millisecond)
	d.bus.Write([]byte{0x00, 0x02}) // home
	time.Sleep(2 * time.Millisecond)
}

func (d *AQM1602Y) DisplayOff() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bus.Write([]byte{0x00, 0x08}) // display off
}

func (d *AQM1602Y) DisplayOn() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bus.Write([]byte{0x00, 0x0c}) // display on
}

func (d *AQM1602Y) SetBuffer(m string) {
	t := []byte(m)
	l := d.UTF8toOLED(&t)
	
	d.display_buff = t[:l]
	d.display_buff_pos = 0
	d.display_buff_len = l

	if d.display_buff_len > 16 {
		d.display_buff = append(d.display_buff, []byte{0x20}...)
		d.display_buff_len = len(d.display_buff)
	}		
}

/*
 * バッファの内容を表示する。桁溢れする場合はスクロール表示する。
 */
func (d *AQM1602Y) PrintBuffer(y uint8) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if d.display_buff_len < 1 {
		return
	}
	y &= 0x01
	d.bus.Write([]byte{0x00, 0x80 + y*0x20}) // set DDRAM address
	time.Sleep(10 * time.Millisecond)
	pos := d.display_buff_pos
	var i int
	for i = 0;i <= 15 && i < d.display_buff_len; i++ {
		d.bus.Write([]byte{0x40, d.display_buff[pos]})
		pos++
		if pos >= d.display_buff_len {
			pos = 0
		}
	}
	for ; i <= 15; i++ {
		d.bus.Write([]byte{0x40, 0x20})
	}
	if d.display_buff_len > 16 {
		d.display_buff_pos++
		if d.display_buff_pos >= d.display_buff_len {
			d.display_buff_pos = 0
		}
	}
}

func (d *AQM1602Y) PrintWithPos(x uint8, y uint8, s []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	x &= 0x0f
	y &= 0x01
	d.bus.Write([]byte{0x00, 0x80 + y*0x20 + x}) // set DDRAM address
	time.Sleep(10 * time.Millisecond)

	d.bus.Write(append([]byte{0x40}, s...))
}


