#! /usr/bin/env python3
import requests, re, socket
from sys import argv
ip = re.compile("<ip>(.+?)</ip>")

# URLを指定
url = "https://playerservices.streamtheworld.com/api/livestream?station={0}&transports=http,hls&version=1.8".format(argv[1])

# リクエスト送信
response = requests.get(url)

pos = 0
m = re.search(ip,response.text)
if m != None:
    s2='{%s}\n' % '\"command\": [\"loadfile\", \"http://{0}/{1}.mp3"]'.format(m.group(1),argv[1])
    # ~ print(s2)
else:
    s2='{%s}\n' % '\"command\": [\"loadfile\", \"http://{0}.mp3"]'.format(argv[1])

s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.connect('/run/mpvsocket')
s.send(s2.encode())
d = s.recv(1024)
s.close()

