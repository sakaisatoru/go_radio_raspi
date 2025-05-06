package netradio

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/carlmjohnson/requests"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"encoding/xml"
)

type StationInfo struct {
	Name string
	Url  string
}

const (
	auth_key  string = "bcd151073c03b352e1ef2fd66c32209da9ca0afa" // 現状は固有 key_lenght = 0
	tokenfile string = "/run/user/1000/radiko_token"
	afnurlfile string = "/run/user/1000/afnurl"
)

type Ports struct {
	XMLName xml.Name `xml:"ports"`
	Port    []string `xml:"port"`
}

type Status struct {
	XMLName        xml.Name `xml:"status"`
	Status_code    int      `xml:"status-code"`
	Status_message string   `xml:"status-message"`
}

type Metadata struct {
	XMLName      xml.Name `xml:"metadata"`
	Shoutcast_v1 string   `xml:"shoutcast-v1"`
	Shoutcast_v2 string   `xml:"shoutcast-v2"`
	Sse_sideband string   `xml:"sse-sideband"`
}

type Audio struct {
	XMLName xml.Name `xml:"audio"`
	Codec   string   `xml:"codec,attr"`
}

type Media_format struct {
	XMLName xml.Name `xml:"media-format"`
	Audio   Audio    `xml:"audio"`
}

type Transports struct {
	XMLName   xml.Name `xml:"transports"`
	Transport []string `xml:"transport"`
}

type Server struct {
	XMLName xml.Name `xml:"server"`
	Ip      string   `xml:"ip"`
	Ports   Ports    `xml:"ports"`
}

type Servers struct {
	XMLName xml.Name `xml:"servers"`
	Server  []Server `xml:"server"`
}

type Mountpoint struct {
	XMLName        xml.Name     `xml:"mountpoint"`
	Status         Status       `xml:"status"`
	Tr             Transports   `xml:"transports"`
	Me             Metadata     `xml:"metadata"`
	Servers        Servers      `xml:"servers"`
	Mount          string       `xml:"mount"`
	Format         string       `xml:"format"`
	Bitrate        int          `xml:"bitrate"`
	MediaFormat    Media_format `xml:"media-format"`
	Authentication int          `xml:"authentication"`
	Timeout        int          `xml:"timeout"`
	Send_page_url  int          `xml:"send-page-url"`
}

type Mountpoints struct {
	XMLName xml.Name     `xml:"mountpoints"`
	Mp      []Mountpoint `xml:"mountpoint"`
}

type Live_stream_config struct {
	XMLName xml.Name `xml:"live_stream_config"`
	Xmlns   string   `xml:"xmlns,attr"`
	Version string   `xml:"version,attr"`
}

type afnfeed struct {
	Lsc Live_stream_config `xml:"live_stream_config"`
	Mps Mountpoints        `xml:"mountpoints"`
}

func PrepareStationList(st string) ([]*StationInfo, error) {
	var (
		file   *os.File
		err    error
		stlist []*StationInfo
	)
	file, err = os.Open(st)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	f := false
	s := ""
	name := ""
	extflag := false

	for scanner.Scan() {
		s = strings.TrimLeft(scanner.Text(), " ")
		if strings.Contains(s, "#EXTM3U") {
			extflag = true
			continue
		}
		if strings.Contains(s, "#EXTINF:") && extflag {
			f = true
			_, name, _ = strings.Cut(s, "/")
			name = strings.Trim(name, " ")
			continue
		}
		if len(s) != 0 {
			if s[:1] == "#" {
				continue
			}
			stmp := new(StationInfo)
			stmp.Url = s
			if f {
				f = false
				// UTF-8 対応で rune　で数える
				stmp.Name = string([]rune(name + "                ")[:16])
			} else {
				stmp.Name = ""
			}
			stlist = append(stlist, stmp)
		}
	}
	return stlist, err
}

func gen_temp_chunk_m3u8_url(url string, auth_token string) (string, error) {
	var (
		chunkurl string
		err      error
	)

	headers := make(http.Header)
	headers.Add("X-Radiko-AuthToken", auth_token)

	h2 := http.Header{}
	var s string
	err = requests.
		URL(url).
		Headers(headers).
		CopyHeaders(h2).
		ToString(&s).
		Fetch(context.Background())
	if err == nil {
		re := regexp.MustCompile(`https?://.+m3u8`)
		chunkurl = re.FindString(s)
	} else {
		chunkurl = ""
	}
	return chunkurl, err
}

var (
	afn_url_cache = map[string]string{"": ""}
)

func AFN_get_url_with_api(station string) (string, error) {
	var s string
	u := ""
	url := fmt.Sprintf("https://playerservices.streamtheworld.com/api/livestream?station=%s&transports=http,hls&version=1.8", station)
	err := requests.
		URL(url).
		ToString(&s).
		Fetch(context.Background())

	lsc := afnfeed{}
	if err == nil {
		e := xml.Unmarshal([]byte(s), &lsc)
		if e == nil {
			for _, mountpoint := range lsc.Mps.Mp {
				if mountpoint.MediaFormat.Audio.Codec == "mp3" {
					t, _ := os.ReadFile(afnurlfile)
					cacheurl := string(t)
					newurl := mountpoint.Servers.Server[0].Ip
					for _, v := range mountpoint.Servers.Server {
						if v.Ip == cacheurl {
							newurl = cacheurl
							break
						}
					}
					u = fmt.Sprintf("https://%s/%s.mp3", newurl, station)
					os.WriteFile(afnurlfile, []byte(newurl), 0666)
					break
				}
			}
		}
	}
	return u, err
}

func Radiko_get_url(station string) (string, error) {
	var (
		authtoken string
		chunkurl  string
		err       error = nil
	)

	station_url := fmt.Sprintf("http://f-radiko.smartstream.ne.jp/%s/_definst_/simul-stream.stream/playlist.m3u8", station)

	t, _ := os.ReadFile(tokenfile)
	authtoken = string(t)
	chunkurl, err = gen_temp_chunk_m3u8_url(station_url, authtoken)

	if err != nil || len(chunkurl) == 0 {
		url := "https://radiko.jp/v2/api/auth1"

		h := make(http.Header)
		h.Add("User-Agent", "curl/7.56.1")
		h.Add("Accept", "*/*")
		h.Add("X-Radiko-App", "pc_html5")
		h.Add("X-Radiko-App-Version", "0.0.1")
		h.Add("X-Radiko-User", "dummy_user")
		h.Add("X-Radiko-Device", "pc")

		h2 := http.Header{}
		var s string
		err := requests.
			URL(url).
			Headers(h).
			CopyHeaders(h2).
			ToString(&s).
			Fetch(context.Background())
		if err != nil {
			//~ fmt.Println("Error ",err)
			goto exit_this
		}

		authtoken = h2.Get("x-radiko-authtoken")
		offset, _ := strconv.Atoi(h2.Get("x-radiko-keyoffset"))
		length, _ := strconv.Atoi(h2.Get("x-radiko-keylength"))
		partialkey := base64.StdEncoding.EncodeToString([]byte(auth_key[offset : offset+length]))

		//~ fmt.Println("authtoken update.")
		os.WriteFile(tokenfile, []byte(authtoken), 0666)

		url2 := "https://radiko.jp/v2/api/auth2"
		h3 := make(http.Header)
		h3.Add("X-Radiko-AuthToken", authtoken)
		h3.Add("X-Radiko-Partialkey", partialkey)
		h3.Add("X-Radiko-User", "dummy_user")
		h3.Add("X-Radiko-Device", "pc")

		h4 := http.Header{}
		var ss string // ss にはリージョンが入る
		err = requests.
			URL(url2).
			Headers(h3).
			CopyHeaders(h4).
			ToString(&ss).
			Fetch(context.Background())
		if err != nil {
			//~ fmt.Println("Error ",err)
			goto exit_this
		}
		chunkurl, err = gen_temp_chunk_m3u8_url(station_url, authtoken)
	}
exit_this:
	return chunkurl, err
}

func Radiko_setup(stlist []*StationInfo) {
	for _, st := range stlist {
		args := strings.Split(st.Url, "/")
		if args[0] == "plugin:" {
			if args[1] == "radiko.py" {
				_, _ = Radiko_get_url(args[2])
				break
			}
		}
	}
}
