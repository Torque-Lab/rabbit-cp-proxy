package tcp_pipe

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"rabbit-cp-proxy/amqphandshake"
	"rabbit-cp-proxy/control_plane"
	"sync"
)

var proxy_user = os.Getenv("PROXY_USER")
var proxy_pass = os.Getenv("PROXY_PASS")

func HandleClient(client net.Conn) {
	defer client.Close()

	// Step 1: Proxy ↔ Client
	err := amqphandshake.ReceiveAMQPHeader(client)
	if err != nil {
		log.Println("failed to receive AMQP header:", err)
		return
	}
	err = amqphandshake.SendConnectionStart(client)
	if err != nil {
		log.Println("failed to send Connection.Start:", err)
		return
	}
	_, username, password, err := amqphandshake.ReadConnectionStartOk(client)
	if err != nil {
		log.Println("failed to read Connection.Start-Ok:", err)
		return
	}
	fmt.Println("Client to Proxy: Received StartOk: user=" + username + " password=" + password)

	backendAddr, err := control_plane.GetBackendAddress(username, password)
	if err != nil {
		log.Println("failed to get backend address:", err)
		amqphandshake.SendConnectionClose(client, 403, "ACCESS_REFUSED")
		return
	}
	if backendAddr == "" {
		amqphandshake.SendConnectionClose(client, 403, "ACCESS_REFUSED")
		log.Println("no backend found for user:", username)
		return
	}
	err = amqphandshake.SendConnectionTune(client)
	if err != nil {
		log.Println("failed to send Connection.Tune:", err)
		return
	}
	err = amqphandshake.ReadConnectionTuneOk(client)
	if err != nil {
		log.Println("failed to read Connection.Tune-Ok:", err)
		return
	}
	err = amqphandshake.ReadConnectionOpen(client)
	if err != nil {
		log.Println("failed to read Connection.Open:", err)
		return
	}
	err = amqphandshake.SendConnectionOpenOk(client)
	if err != nil {
		log.Println("failed to send Connection.Open-Ok:", err)
		return
	}

	if backendAddr != "" {
		backendAddr = "rabbitmq:5672"
	}
	rabbit, err := net.Dial("tcp4", backendAddr)
	if err != nil {
		log.Println("failed to connect backend:", err)
		return
	}

	defer rabbit.Close()
	// Step 2: Proxy ↔ Real Rabbit
	err = amqphandshake.SendAMQPHeader(rabbit)
	if err != nil {
		amqphandshake.SendConnectionClose(client, 403, "CONNECTION_ERROR")
		log.Println("failed to send AMQP header:", err)
		return
	}
	_, err = amqphandshake.ReadConnectionStart(rabbit)
	if err != nil {
		amqphandshake.SendConnectionClose(client, 403, "CONNECTION_ERROR")
		log.Println("failed to read Connection.Start:", err)
		return
	}
	err = amqphandshake.SendConnectionStartOk(rabbit, proxy_user, proxy_pass)
	if err != nil {
		amqphandshake.SendConnectionClose(client, 403, "CONNECTION_ERROR")
		log.Println("failed to send Connection.Start-Ok:", err)
		return
	}
	_, err = amqphandshake.ReadConnectionTune(rabbit)
	if err != nil {
		amqphandshake.SendConnectionClose(client, 403, "CONNECTION_ERROR")
		log.Println("failed to read Connection.Tune:", err)
		return
	}
	err = amqphandshake.SendConnectionTuneOk(rabbit)
	if err != nil {
		amqphandshake.SendConnectionClose(client, 403, "CONNECTION_ERROR")
		log.Println("failed to send Connection.Tune-Ok:", err)
		return
	}
	err = amqphandshake.SendConnectionOpen(rabbit, "/")
	if err != nil {
		amqphandshake.SendConnectionClose(client, 403, "CONNECTION_ERROR")
		log.Println("failed to send Connection.Open:", err)
		return
	}
	err = amqphandshake.ReadConnectionOpenOk(rabbit)
	if err != nil {
		amqphandshake.SendConnectionClose(client, 403, "CONNECTION_ERROR")
		log.Println("failed to read Connection.Open-Ok:", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go proxyPipe(client, rabbit, &wg) // copy client -> backend
	go proxyPipe(rabbit, client, &wg) // copy backend -> client
	wg.Wait()

}

func proxyPipe(src, dst net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	_, _ = io.Copy(dst, src)
	if tcpConn, ok := dst.(*net.TCPConn); ok {
		_ = tcpConn.CloseWrite()
	}
	if tcpConn, ok := src.(*net.TCPConn); ok {
		_ = tcpConn.CloseRead()
	}
}
