package main

import (
	"crypto/tls"
	"log"
	"os"
	"rabbit-cp-proxy/control_plane"
	_ "rabbit-cp-proxy/env_config"
	"rabbit-cp-proxy/tcp_pipe"
)

var proxy_port = os.Getenv("PROXY_PORT")

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
	ln, err := tls.Listen("tcp", ":"+proxy_port, tlsConfig)
	if err != nil {
		log.Println("TLS listen error:", err)
		return
	}
	log.Printf("RabbitMQ proxy listening on:%s", proxy_port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("accept error:", err)
			continue
		}
		go tcp_pipe.HandleClient(conn)
	}
}
