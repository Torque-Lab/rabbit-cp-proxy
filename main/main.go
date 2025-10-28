package main

import (
	"crypto/tls"
	"log"
	"rabbit-cp-proxy/control_plane"
	"rabbit-cp-proxy/tcp_pipe"
)

func main() {
	certPath := "/etc/ssl/certs/tls.crt"
	keyPath := "/etc/ssl/certs/tls.key"
	go control_plane.StartUpdateServer()

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		log.Println("failed to load TLS cert:", err)
		return
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}

	ln, err := tls.Listen("tcp", ":5671", tlsConfig)
	if err != nil {
		log.Println("TLS listen error:", err)
		return
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
