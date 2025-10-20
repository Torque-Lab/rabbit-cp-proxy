package main

import (
	"crypto/tls"
	"log"
	"rabbit-cp-proxy/control_plane"
	"rabbit-cp-proxy/tcp_pipe"
)

func main() {
	go control_plane.StartUpdateServer()

	cert, err := tls.LoadX509KeyPair("/etc/ssl/certs/wildcard.crt", "/etc/ssl/certs/wildcard.key")
	if err != nil {
		log.Fatal("failed to load TLS cert:", err)
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}

	ln, err := tls.Listen("tcp", ":5671", tlsConfig)
	if err != nil {
		log.Fatal("TLS listen error:", err)
	}
	log.Println("RabbitMQ proxy listening on :5671")

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("accept error:", err)
			continue
		}
		go tcp_pipe.HandleClient(conn)
	}
}
