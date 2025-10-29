package amqphandshake

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
)

// Constant frame end byte
const frameEnd byte = 0xCE

// CLIENT-SIDE (Proxy -> Rabbit)
func SendAMQPHeader(c net.Conn) error {
	header := []byte{'A', 'M', 'Q', 'P', 0, 0, 9, 1}
	_, err := c.Write(header)
	if err == nil {
		log.Println("Proxy to Rabbit: Sent AMQP header")
	}
	return err
}

func ReadConnectionStart(c net.Conn) ([]byte, error) {
	frame, err := ReadFrame(c)
	if err != nil {
		return nil, fmt.Errorf("failed to read Connection.Start: %w", err)
	}
	log.Println("← Proxy to Rabbit talk: client received Connection.Start")
	return frame, nil
}
func SendConnectionStartOk(c net.Conn, username, password string) error {
	var payload bytes.Buffer

	// Class 10 (Connection), Method 11 (StartOk)
	binary.Write(&payload, binary.BigEndian, uint16(10))
	binary.Write(&payload, binary.BigEndian, uint16(11))

	// client-properties: empty table
	binary.Write(&payload, binary.BigEndian, uint32(0))

	// mechanism: short string "PLAIN"
	mech := []byte("PLAIN")
	payload.WriteByte(byte(len(mech))) // short-str → 1 byte length
	payload.Write(mech)

	// response: long string [0 user 0 pass]
	auth := []byte{0}
	auth = append(auth, []byte(username)...)
	auth = append(auth, 0)
	auth = append(auth, []byte(password)...)
	binary.Write(&payload, binary.BigEndian, uint32(len(auth))) // long-str → 4 bytes
	payload.Write(auth)

	loc := []byte("en_US")
	payload.WriteByte(byte(len(loc))) // short-str → 1 byte
	payload.Write(loc)

	fmt.Println("StartOk payload hex:", hex.EncodeToString(payload.Bytes()))

	return WriteMethodFrame(c, 0, payload.Bytes(), "Proxy to Rabbit: Sent Connection.StartOk")
}

func ReadConnectionTune(c net.Conn) ([]byte, error) {
	frame, err := ReadFrame(c)
	if err != nil {
		return nil, fmt.Errorf("failed to read Connection.Tune: %w", err)
	}
	log.Println(" Proxy to Rabbit: Received Connection.Tune")
	return frame, nil
}

func SendConnectionTuneOk(c net.Conn) error {
	var payload bytes.Buffer

	binary.Write(&payload, binary.BigEndian, uint16(10))
	binary.Write(&payload, binary.BigEndian, uint16(31))     // Tune-Ok
	binary.Write(&payload, binary.BigEndian, uint16(1024))   // channel-max
	binary.Write(&payload, binary.BigEndian, uint32(131072)) // frame-max (128KB)
	binary.Write(&payload, binary.BigEndian, uint16(60))     // heartbeat
	fmt.Println("send connection tune ok payload hex:", hex.EncodeToString(payload.Bytes()))
	return WriteMethodFrame(c, 0, payload.Bytes(), "Proxy to Rabbit: Sent Connection.TuneOk")
}

func SendConnectionOpen(c net.Conn, vhost string) error {
	var payload bytes.Buffer
	binary.Write(&payload, binary.BigEndian, uint16(10))
	binary.Write(&payload, binary.BigEndian, uint16(40)) // Connection.Open
	payload.WriteByte(byte(len(vhost)))
	payload.WriteString(vhost)
	payload.WriteByte(0) // reserved-1
	payload.WriteByte(0) // insist=false

	return WriteMethodFrame(c, 0, payload.Bytes(), "Proxy to Rabbit: Sent Connection.Open")
}

func ReadConnectionOpenOk(c net.Conn) error {
	frame, err := ReadFrame(c)
	if err != nil {
		return fmt.Errorf("failed to read Connection.Open-Ok: %w", err)
	}
	log.Println(" Proxyto Rabbit: Received Connection.OpenOk")
	_ = frame
	return nil
}

// SERVER-SIDE (Client -> Proxy)

func ReceiveAMQPHeader(c net.Conn) error {
	header := make([]byte, 8)
	if _, err := io.ReadFull(c, header); err != nil {
		return fmt.Errorf("failed to read AMQP header: %w", err)
	}
	log.Println("Client to Proxy: Received AMQP header:", string(header))
	return nil
}

func SendConnectionStart(c net.Conn) error {
	var payload bytes.Buffer
	binary.Write(&payload, binary.BigEndian, uint16(10))
	binary.Write(&payload, binary.BigEndian, uint16(10)) // Connection.Start
	payload.WriteByte(0)                                 // version-major
	payload.WriteByte(9)                                 // version-minor

	// server-properties (empty)
	binary.Write(&payload, binary.BigEndian, uint32(0))

	// mechanisms: longstr "PLAIN "
	mech := []byte("PLAIN ")
	binary.Write(&payload, binary.BigEndian, uint32(len(mech)))
	payload.Write(mech)

	loc := []byte("en_US")
	binary.Write(&payload, binary.BigEndian, uint32(len(loc)))
	payload.Write(loc)

	return WriteMethodFrame(c, 0, payload.Bytes(), "Client to Proxy: Sent Connection.Start")
}

func ReadConnectionStartOk(c net.Conn) ([]byte, string, string, error) {
	frame, err := ReadFrame(c)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to read Connection.StartOk: %w", err)
	}

	payload := frame[7 : len(frame)-1]
	username, password, err := extractPlainAuthFromStartOk(payload)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to extract auth from StartOk: %w", err)
	}
	return frame, username, password, nil

}

func SendConnectionTune(c net.Conn) error {
	var payload bytes.Buffer
	binary.Write(&payload, binary.BigEndian, uint16(10))     // class-id
	binary.Write(&payload, binary.BigEndian, uint16(30))     // method-id (Tune)
	binary.Write(&payload, binary.BigEndian, uint16(0))      // channel-max
	binary.Write(&payload, binary.BigEndian, uint32(131072)) // frame-max
	binary.Write(&payload, binary.BigEndian, uint16(0))      // heartbeat

	return WriteMethodFrame(c, 0, payload.Bytes(), "Client to Proxy: Sent Connection.Tune")
}

func ReadConnectionTuneOk(c net.Conn) error {
	frame, err := ReadFrame(c)
	if err != nil {
		return fmt.Errorf("failed to read TuneOk: %w", err)
	}
	log.Println("Client to Proxy: Received TuneOk")
	_ = frame
	return nil
}

func ReadConnectionOpen(c net.Conn) error {
	frame, err := ReadFrame(c)
	if err != nil {
		return fmt.Errorf("failed to read Connection.Open: %w", err)
	}
	log.Println("Client to Proxy: Received Connection.Open")
	_ = frame
	return nil
}

func SendConnectionOpenOk(c net.Conn) error {
	var payload bytes.Buffer
	binary.Write(&payload, binary.BigEndian, uint16(10))
	binary.Write(&payload, binary.BigEndian, uint16(41)) // Open-Ok
	binary.Write(&payload, binary.BigEndian, uint8(0))   // reserved-1

	return WriteMethodFrame(c, 0, payload.Bytes(), "Client to Proxy: Sent Connection.OpenOk")
}

func ReadFrame(c net.Conn) ([]byte, error) {
	header := make([]byte, 7)
	if _, err := io.ReadFull(c, header); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(header[3:7])
	payload := make([]byte, size+1)
	if _, err := io.ReadFull(c, payload); err != nil {
		return nil, err
	}
	return append(header, payload...), nil
}

func extractPlainAuthFromStartOk(payload []byte) (string, string, error) {
	if len(payload) < 4 {
		return "", "", fmt.Errorf("payload too short")
	}
	classID := binary.BigEndian.Uint16(payload[0:2])
	methodID := binary.BigEndian.Uint16(payload[2:4])
	if classID != 10 || methodID != 11 { // start-ok is class 10 method 11
		log.Printf("warning: Start-Ok expected (10,11) got (%d,%d). Inspecting payload.", classID, methodID)
	}

	idx := bytes.Index(payload, []byte("PLAIN"))
	if idx == -1 {
		return "", "", fmt.Errorf("no PLAIN mechanism found in payload (dump=%s)", hex.EncodeToString(payload))
	}
	for off := idx + 5; off+4 < len(payload); off++ {
		l := int(binary.BigEndian.Uint32(payload[off : off+4]))
		start := off + 4
		end := start + l
		if end > len(payload) {
			continue
		}
		data := payload[start:end]
		parts := bytes.SplitN(data, []byte{0x00}, 3)
		if len(parts) >= 3 {
			username := string(parts[1])
			password := string(parts[2])
			return username, password, nil
		}
		off = end - 1
	}
	return "", "", fmt.Errorf("no SASL response containing NUL-separated username/password found (dump=%s)", hex.EncodeToString(payload))
}
func WriteMethodFrame(c net.Conn, channel uint16, payload []byte, logMsg string) error {
	var frame bytes.Buffer
	frame.WriteByte(1)
	binary.Write(&frame, binary.BigEndian, channel)
	binary.Write(&frame, binary.BigEndian, uint32(len(payload)))
	frame.Write(payload)
	frame.WriteByte(frameEnd)
	fmt.Println("StartOk in write hex:", hex.EncodeToString(frame.Bytes()))
	_, err := c.Write(frame.Bytes())
	if err == nil {
		log.Println(logMsg)
	}
	return err
}

func SendConnectionClose(conn net.Conn, replyCode uint16, reason string) error {
	const (
		classID       = 10
		methodID      = 50
		failingClass  = 0
		failingMethod = 0
	)

	payload := make([]byte, 0)
	payload = append(payload,
		byte(classID>>8), byte(classID),
		byte(methodID>>8), byte(methodID),
		byte(replyCode>>8), byte(replyCode&0xFF),
	)

	if len(reason) > 255 {
		reason = reason[:255]
	}
	payload = append(payload, byte(len(reason)))
	payload = append(payload, []byte(reason)...)

	payload = append(payload,
		byte(failingClass>>8), byte(failingClass),
		byte(failingMethod>>8), byte(failingMethod),
	)

	frame := buildAMQPFrame(1, 0, payload)
	if _, err := conn.Write(frame); err != nil {
		return fmt.Errorf("failed to send Connection.Close: %w", err)
	}
	log.Println("Sent Connection.Close:", reason)
	return nil
}

func buildAMQPFrame(frameType byte, channel uint16, payload []byte) []byte {
	frame := make([]byte, 0, 7+len(payload)+1)
	frame = append(frame, frameType)
	frame = append(frame, byte(channel>>8), byte(channel))
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(size, uint32(len(payload)))
	frame = append(frame, size...)
	frame = append(frame, payload...)
	frame = append(frame, 0xCE) // frame end
	return frame
}
