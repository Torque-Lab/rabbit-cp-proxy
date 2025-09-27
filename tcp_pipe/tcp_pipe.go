package tcp_pipe

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"rabbit-cp-proxy/control_plane"
	"sync"
)

func HandleClient(client net.Conn) {
	defer client.Close()
	buf := make([]byte, 4096)
	n, err := client.Read(buf)
	if err != nil {
		log.Println("failed to read initial handshake:", err)
		return
	}
	data := buf[:n]

	username, password, err := extractPlainAuth(data)
	if err != nil {
		log.Println("failed to extract username:", err)
		return
	}
	backendAddr, err := control_plane.GetBackendAddress(username, password)
	if err != nil {
		log.Println("failed to get backend address:", err)
		return
	}
	if backendAddr == "" {
		log.Println("no backend found for user:", username)
		return
	}

	rabbit, err := net.Dial("tcp", backendAddr)
	if err != nil {
		log.Println("failed to connect backend:", err)
		return
	}
	defer rabbit.Close()
	if _, err := rabbit.Write(data); err != nil {
		log.Println("failed to send handshake to backend:", err)
		return
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go proxyPipe(client, rabbit)
	go proxyPipe(rabbit, client)
	wg.Wait()
}

func extractPlainAuth(data []byte) (string, string, error) {
	parts := bytes.SplitN(data, []byte{0x00}, 3)
	if len(parts) < 3 {
		return "", "", fmt.Errorf("no SASL PLAIN found")
	}
	username := string(parts[1])
	password := string(parts[2])
	return username, password, nil
}

func proxyPipe(client net.Conn, backendAddr net.Conn) {
	defer client.Close()
	defer backendAddr.Close()
	io.Copy(client, backendAddr)
}
