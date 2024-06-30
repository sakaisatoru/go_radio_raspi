module radio.raspi/radio_r

go 1.22.1

replace local.packages/aqm1602y => ./aqm1602y

replace local.packages/netradio => ./netradio

require (
	github.com/davecheney/i2c v0.0.0-20140823063045-caf08501bef2
	github.com/stianeikeland/go-rpio/v4 v4.6.0
	local.packages/aqm1602y v0.0.0-00010101000000-000000000000
	local.packages/netradio v0.0.0-00010101000000-000000000000
)

require (
	github.com/carlmjohnson/requests v0.23.5 // indirect
	golang.org/x/net v0.15.0 // indirect
)
