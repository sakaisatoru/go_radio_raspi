package main

import (
	"bufio"
	"log"
	"net"
)

const (
	serversocket string = "/run/user/1001/go_radiosocket"
)

func server(ln net.Listener, ch chan<- string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok {
				if opErr.Err.Error() == "use of closed network connection" {
					return
				}
			}
			log.Println("server() ", err)
			continue
		}

		for {
			reader := bufio.NewReader(conn)
			message, err := reader.ReadString('\n')
			if err != nil {
				log.Println("server() ", err)
				break
			}
			ch <- message
		}
	}
}
