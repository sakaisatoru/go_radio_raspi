package main

import (
	"fmt"
	"github.com/sakaisatoru/weatherinfo"
	"log"
	"strings"
)

var (
	forecastinfo_enable bool = false
	forecast_area_ul    *map[string]string
	foreloc             string
)

func setup_forecast(loc string) {
	var err error
	weatherinfo.SetWorkingDir(WEATHER_WORKING_DIR)
	foreloc = loc
	forecast_area_ul, err = weatherinfo.ForecastUrlTargetArea(foreloc)
	if err != nil {
		log.Println("weatherinfo.ForecastUrlTargetArea", err)
	} else {
		forecastinfo_enable = true
	}
	weather_i = weatherinfo.New()
}

func info_forecast() string {
	var (
		label *string
		fore  *weatherinfo.Forecast
		rs    string
	)
	if forecastinfo_enable == false {
		rs = "ｹﾞﾝｻﾞｲﾃﾝｷﾖﾎｳﾊｼｭﾄｷﾃﾞｷﾏｾﾝ"
	} else {
		switch display_info {
		case display_info_weather_1:
			if err := weather_i.GetWeatherInfo((*forecast_area_ul)[foreloc], foreloc); err == nil {
				// 警報・注意報
				for i := 0; i < len(weather_i.Warning); i++ {
					al := strings.Split(weather_i.Warning[i].AlarmType, "、")
					for j := 0; j < len(al); j++ {
						al[j] = weatherinfo.KanaName[al[j]]
					}
					stmp := strings.TrimRight(strings.Join(al, ","), ",")
					rs = fmt.Sprintf("%s %s\n",
						weatherinfo.KanaName[weather_i.Warning[i].Label], stmp)
				}
			} else {
				// 天気予報取得失敗
				display_info = display_info_default
				rs = errmessage[DIR_NOT_READY]
			}

		case display_info_weather_2, display_info_weather_3,
			display_info_weather_4, display_info_weather_5:
			after_hour := []int{1, 6, 12, 18}
			label, fore = weather_i.GetHoursLaterInfo(after_hour[display_info-display_info_weather_2])
			rs = fmt.Sprintf("%s  %s %dﾟC",
				*label,
				weatherinfo.KanaName[fore.Weather], fore.Termperature)
		}
	}
	return rs
}
