module github.com/sakaisatoru/go_radio_raspi

go 1.25.5

replace local.packages/aqm1602y => ./aqm1602y

replace local.packages/irremote => ./irremote

replace local.packages/mpvctl => ./mpvctl

replace local.packages/rotaryencoder => ./rotaryencoder

require (
	github.com/davecheney/i2c v0.0.0-20140823063045-caf08501bef2
	github.com/sakaisatoru/go_mpvradio/netradio v0.0.0-20260627232503-d6bf3b075023
	github.com/sakaisatoru/weatherinfo v0.0.0-20260603031758-977e4f5aa716
	github.com/stianeikeland/go-rpio/v4 v4.6.0
	local.packages/aqm1602y v0.0.0-00010101000000-000000000000
	local.packages/irremote v0.0.0-00010101000000-000000000000
	local.packages/mpvctl v0.0.0-00010101000000-000000000000
	local.packages/rotaryencoder v0.0.0-00010101000000-000000000000
)

require (
	github.com/PuerkitoBio/goquery v1.10.0 // indirect
	github.com/andybalholm/cascadia v1.3.2 // indirect
	github.com/carlmjohnson/requests v0.25.1 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
)
