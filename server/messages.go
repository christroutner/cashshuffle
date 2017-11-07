package server

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"

	"github.com/cashshuffle/cashshuffle/message"
	"github.com/golang/protobuf/proto"
)

const (
	maxMessageLength = 64 * 1024
)

var (
	// breakBytes are the bytes that delimit each protobuf message
	// This represents the character ⏎
	breakBytes = []byte{226, 143, 142}
)

// startSignedChan starts a loop reading messages.
func startSignedChan(c chan *signedConn) {
	for {
		sc := <-c
		err := sc.processReceivedMessage()
		if err != nil {
			sc.conn.Close()
			fmt.Fprintf(os.Stderr, "[Error] %s\n", err.Error())
		}
	}
}

// processReceivedMessage reads the message and processes it.
func (sc *signedConn) processReceivedMessage() error {
	// If we are not tracking the connection yet, the user must be
	// registering with the server.
	if sc.tracker.getTrackerData(sc.conn) == nil {
		err := sc.registerClient()
		if err != nil {
			return err
		}

		playerData := sc.tracker.getTrackerData(sc.conn)

		if sc.tracker.getPoolSize(playerData.pool) == sc.tracker.poolSize {
			sc.announceStart()
		}

		return nil
	}

	if err := sc.verifyMessage(); err != nil {
		return err
	}

	err := sc.broadcastMessage()
	return err
}

// processMessages reads messages from the connection and begins processing.
func processMessages(conn net.Conn, c chan *signedConn, t *tracker) {
	scanner := bufio.NewScanner(conn)
	scanner.Split(bufio.ScanBytes)

	for {
		var b bytes.Buffer

		for scanner.Scan() {
			scanBytes := scanner.Bytes()

			if len(b.String()) > maxMessageLength {
				fmt.Fprintln(os.Stderr, "[Error] message too long")
				return
			}

			b.Write(scanBytes)

			if breakScan(b) {
				b.Truncate(b.Len() - 3)
				break
			}

		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "[Error] %s\n", err.Error())
			break
		}

		// We should not receive empty messages.
		if b.String() == "" {
			break
		}

		if err := sendToSignedChan(&b, conn, c, t); err != nil {
			fmt.Fprintf(os.Stderr, "[Error] %s\n", err.Error())
			break
		}
	}
}

// sendToSignedChannel takes a byte buffer containing a protobuf message,
// converts it to message.Signed and sends it over signedChan.
func sendToSignedChan(b *bytes.Buffer, conn net.Conn, c chan *signedConn, t *tracker) error {
	defer b.Reset()

	pdata := new(message.Packets)

	err := proto.Unmarshal(b.Bytes(), pdata)
	if err != nil {
		if debugMode {
			fmt.Println("[Error] Unmarshal failed:", b.Bytes())
		}
		return err
	}

	if debugMode {
		fmt.Println("[Received]", pdata)
	}

	for _, signed := range pdata.Packet {
		data := &signedConn{
			message: signed,
			conn:    conn,
			tracker: t,
		}

		c <- data
	}

	return nil
}

// breakScan checks if a byte sequence is the break point on the scanner.
func breakScan(buf bytes.Buffer) bool {
	len := buf.Len()

	if len > 3 {
		payload := buf.Bytes()
		bs := []byte{
			payload[len-3],
			payload[len-2],
			payload[len-1],
		}

		for i := range bs {
			if bs[i] != breakBytes[i] {
				return false
			}
		}

		return true
	}

	return false
}
