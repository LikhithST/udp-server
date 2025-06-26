package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

type DataChannelMessage struct {
	FrameID                      int64  `json:"frameID"`
	MessageSentTimeLocalMachine1 int64  `json:"messageSentTime_LocalMachine1,omitempty"`
	MessageSentTimeVM1           int64  `json:"messageSentTime_VM1,omitempty"`
	MessageSentTimeVM2           int64  `json:"messageSentTime_VM2,omitempty"`
	MessageSentTimeLocalMachine2 int64  `json:"messageSentTime_LocalMachine2,omitempty"`
	LatencyEndToEnd              int64  `json:"latency_end_to_end,omitempty"`
	MessageSendRate              int64  `json:"message_send_rate,omitempty"`
	Payload                      []byte `json:"payload"`
}

func main() {
	// Command-line flags for listen and forward addresses
	listenAddr := flag.String("listen", ":12345", "UDP listen address (e.g. :12345)")
	forwardAddr := flag.String("forward", "127.0.0.1:23456", "UDP forward address (e.g. 127.0.0.1:23456)")
	bufferSize := flag.Int("buffer", 1400, "Buffer size for incoming messages")
	flag.Parse()

	// Resolve addresses
	laddr, err := net.ResolveUDPAddr("udp", *listenAddr)
	if err != nil {
		fmt.Println("Failed to resolve listen address:", err)
		os.Exit(1)
	}

	faddr, err := net.ResolveUDPAddr("udp", *forwardAddr)
	if err != nil {
		fmt.Println("Failed to resolve forward address:", err)
		os.Exit(1)
	}

	// Listen for incoming UDP packets
	listener, err := net.ListenUDP("udp", laddr)
	if err != nil {
		fmt.Println("Failed to listen:", err)
		os.Exit(1)
	}
	defer listener.Close()

	// Prepare UDP connection for forwarding
	forwardConn, err := net.DialUDP("udp", nil, faddr)
	if err != nil {
		fmt.Println("Failed to dial forward address:", err)
		os.Exit(1)
	}
	defer forwardConn.Close()

	buf := make([]byte, *bufferSize) // Buffer size for incoming messages
	for {
		n, clientAddr, err := listener.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Read error:", err)
			continue
		}

		var msg DataChannelMessage
		err = json.Unmarshal(buf[:n], &msg)
		// fmt.Println("Received message:", buf)
		fmt.Printf("Received message:%#v \n", msg)
		if err != nil {
			fmt.Println("JSON unmarshal error:", err)
			continue
		}
		msg.MessageSentTimeVM1 = time.Now().UnixMilli()

		data, err := json.Marshal(msg)
		if err != nil {
			fmt.Println("Error marshaling message:", err)
			continue
		}
		// if err := json.NewEncoder(&buf).Encode(msg); err != nil {
		// 	fmt.Println("Failed to encode payload:", err)
		// 	os.Exit(1)
		// }

		fmt.Printf("Received %d bytes from %s, forwarding to %s\n", n, clientAddr, faddr)
		_, err = forwardConn.Write(data)
		if err != nil {
			fmt.Println("Forward error:", err)
		}
	}
}
