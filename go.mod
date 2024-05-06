module radio.raspi/radio_r

go 1.22.1

replace local.packages/aqm1602y => ./aqm1602y

require (
	github.com/davecheney/i2c v0.0.0-20140823063045-caf08501bef2
	github.com/stianeikeland/go-rpio/v4 v4.6.0
	local.packages/aqm1602y v0.0.0-00010101000000-000000000000
)
