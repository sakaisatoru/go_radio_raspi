package rotaryencoder

import (
	"github.com/stianeikeland/go-rpio/v4"
	"time"
)

type REvector int

const (
	NoData REvector = iota
	Forward
	Backward
)

type RotaryEncoder struct {
	pin_a        rpio.Pin
	pin_b        rpio.Pin
	counter      int
	samplingtime int
	cb_forward   func()
	cb_backward  func()
}

func cb_default() {
}

func New(a rpio.Pin, b rpio.Pin, cb_for func(), cb_back func()) RotaryEncoder {
	if cb_for == nil {
		cb_for = cb_default
	}
	if cb_back == nil {
		cb_for = cb_default
	}
	return RotaryEncoder{
		pin_a:        a,
		pin_b:        b,
		counter:      0,
		samplingtime: 2,
		cb_forward:   cb_for,
		cb_backward:  cb_back,
	}
}

func (r *RotaryEncoder) Init() {
	r.ResetCounter()
}

func (r *RotaryEncoder) ResetCounter() {
	r.counter = 0
}

func (r *RotaryEncoder) GetCounter() int {
	return r.counter
}

func (r *RotaryEncoder) SetCounter(n int) int {
	r.counter = n
	return r.counter
}

func (r *RotaryEncoder) SetSamplingTime(n int) int {
	r.samplingtime = n
	return r.samplingtime
}

func (r *RotaryEncoder) DetectLoop(code chan<- REvector) {
	var (
		a_edgemode rpio.Edge
		b_edgemode rpio.Edge
	)
	a_edgemode = rpio.FallEdge
	rpio.Pin(r.pin_a).Detect(a_edgemode) // A相たち下がり検出
	b_edgemode = rpio.NoEdge
	rpio.Pin(r.pin_b).Detect(b_edgemode) // B相エッジ検出マスク

	for {
		time.Sleep(time.Duration(r.samplingtime) * time.Millisecond)
		if rpio.Pin(r.pin_a).EdgeDetected() {
			if a_edgemode == rpio.FallEdge {
				// A相たち下がりを検出
				a_edgemode = rpio.NoEdge
				rpio.Pin(r.pin_a).Detect(a_edgemode) // エッジ検出をマスク
				if rpio.Pin(r.pin_b).Read() == 1 {
					// cw
					b_edgemode = rpio.FallEdge
					rpio.Pin(r.pin_b).Detect(b_edgemode) // B相たち下がり検出設定
				} else {
					// ccw
					b_edgemode = rpio.RiseEdge
					rpio.Pin(r.pin_b).Detect(b_edgemode) // B相たち上がり検出設定
				}
			}
			if a_edgemode == rpio.RiseEdge {
				// A相立ち上がりを検出
				a_edgemode = rpio.NoEdge
				rpio.Pin(r.pin_a).Detect(a_edgemode) // エッジ検出をマスク
				if rpio.Pin(r.pin_b).Read() == 0 {
					// cw
					b_edgemode = rpio.RiseEdge
					rpio.Pin(r.pin_b).Detect(b_edgemode) // B相たち上がり検出設定
					r.counter++
					r.cb_forward()
					code <- Forward
				} else {
					// ccw
					b_edgemode = rpio.FallEdge
					rpio.Pin(r.pin_b).Detect(b_edgemode) // B相たち下がり検出設定
					r.counter--
					r.cb_backward()
					code <- Backward
				}
			}
		}
		if rpio.Pin(r.pin_b).EdgeDetected() {
			if b_edgemode == rpio.FallEdge {
				// B相たち下がりを検出
				b_edgemode = rpio.NoEdge
				rpio.Pin(r.pin_b).Detect(b_edgemode) // エッジ検出をマスク
				if rpio.Pin(r.pin_a).Read() == 1 {
					// ccw
					a_edgemode = rpio.FallEdge
					rpio.Pin(r.pin_a).Detect(a_edgemode) // A相たち下がり設定
				} else {
					// cw
					a_edgemode = rpio.RiseEdge
					rpio.Pin(r.pin_a).Detect(a_edgemode) // A相たち上がり設定
				}
			}
			if b_edgemode == rpio.RiseEdge {
				// B相立ち上がりを検出
				b_edgemode = rpio.NoEdge
				rpio.Pin(r.pin_b).Detect(b_edgemode) // エッジ検出をマスク
				if rpio.Pin(r.pin_a).Read() == 1 {
					// cw
					a_edgemode = rpio.FallEdge
					rpio.Pin(r.pin_a).Detect(a_edgemode) // A相たち下がり設定
				} else {
					// ccw
					a_edgemode = rpio.RiseEdge
					rpio.Pin(r.pin_a).Detect(a_edgemode) // A相たち上がり設定
				}
			}
		}
	}
}
