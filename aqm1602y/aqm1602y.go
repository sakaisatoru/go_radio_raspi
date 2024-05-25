package aqm1602y

import (
	"github.com/davecheney/i2c"
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
}

func New(bus *i2c.I2C) AQM1602Y {
	return AQM1602Y {
		bus:		*bus,
	}
}

func (d *AQM1602Y) Configure() {
	//~ ST7032I_1602_writecommand(0x38);// Function Set, 2 line mode normal display
	//~ ST7032I_1602_writecommand(0x39);// Unction Set, extension instruction be selected
	//~ ST7032I_1602_writecommand(0x14);// BS=0:1/5 BIAS;F2 F1 F0:100(internal osc)
	//~ ST7032I_1602_writecommand(0x73);// Contrast set
	//~ ST7032I_1602_writecommand(0x5E);// Icon on,booster circuit on
	//~ ST7032I_1602_writecommand(0x6C);// Follower circuit on
	//~ ST7032I_1602_writecommand(0x0C);// Entire display on
	//~ ST7032I_1602_writecommand(0x01);// Clear display
	//~ ST7032I_1602_writecommand(0x06);// Entry Mode Set ,increment

	//~ var init []byte = {0x38, 0x39, 0x14, 0x73, 0x5e, 0x6c, 0x0c, 0x01, 0x06}
	//~ for i, r := range init {
		//~ _, err := d.Write(r)
		
	//~ }
	
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

func (d *AQM1602Y) Clear() {
	d.bus.Write([]byte{0x00, 0x01}) // display off
	time.Sleep(20 * time.Millisecond)
	d.bus.Write([]byte{0x00, 0x02}) // home
	time.Sleep(2 * time.Millisecond)
}

func (d *AQM1602Y) DisplayOff() {
	d.bus.Write([]byte{0x00, 0x08}) // display off
}

func (d *AQM1602Y) DisplayOn() {
	d.bus.Write([]byte{0x00, 0x0c}) // display on
}

func (d *AQM1602Y) PrintWithPos(x uint8, y uint8, s []byte) {
	x &= 0x0f
	y &= 0x01
	d.bus.Write([]byte{0x00, 0x80 + y*0x20 + x}) // set DDRAM address
	time.Sleep(10 * time.Millisecond)

	d.bus.Write(append([]byte{0x40}, s...))
}


