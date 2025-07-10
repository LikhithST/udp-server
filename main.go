package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
)

type DataChannelMessage struct {
	FrameID                        int64   `json:"frameID"`
	MessageSentTimeLocalMachine1   int64   `json:"messageSentTime_LocalMachine1,omitempty"`
	CPUPercentLocalMachine1        float64 `json:"cpuPercent_LocalMachine1,omitempty"`
	SystemCPUPercentLocalMachine1  float64 `json:"systemCpuPercent_LocalMachine1,omitempty"`
	MemoryUsageLocalMachine1       int64   `json:"memoryUsage_LocalMachine1,omitempty"`
	SystemMemoryUsageLocalMachine1 int64   `json:"systemMemoryUsage_LocalMachine1,omitempty"`
	MessageSentTimeVM1             int64   `json:"messageSentTime_VM1,omitempty"`
	PacketLossVM1                  int64   `json:"packetLoss_VM1,omitempty"`
	CPUPercentVM1                  float64 `json:"cpuPercent_VM1,omitempty"`
	SystemCPUPercentVM1            float64 `json:"systemCpuPercent_VM1,omitempty"`
	MemoryUsageVM1                 int64   `json:"memoryUsage_VM1,omitempty"`
	SystemMemoryUsageVM1           int64   `json:"systemMemoryUsage_VM1,omitempty"`
	MessageSentTimeVM2             int64   `json:"messageSentTime_VM2,omitempty"`
	PacketLossVM2                  int64   `json:"packetLoss_VM2,omitempty"`
	CPUPercentVM2                  float64 `json:"cpuPercent_VM2,omitempty"`
	SystemCPUPercentVM2            float64 `json:"systemCpuPercent_VM2,omitempty"`
	MemoryUsageVM2                 int64   `json:"memoryUsage_VM2,omitempty"`
	SystemMemoryUsageVM2           int64   `json:"systemMemoryUsage_VM2,omitempty"`
	MessageSentTimeLocalMachine2   int64   `json:"messageSentTime_LocalMachine2,omitempty"`
	LatencyEndToEnd                int64   `json:"latency_end_to_end,omitempty"`
	MessageSendRate                int64   `json:"message_send_rate,omitempty"`
	Payload                        []byte  `json:"payload"`
}

func main() {

	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		panic(err)
	}
	fmt.Println("Process ID:", p.Pid)
	// for {
	// 	percent, _ := cpu.Percent(1*time.Second, false)
	// 	fmt.Printf("CPU Usage: %.2f%%\n", percent[0])

	// 	vm, _ := mem.VirtualMemory()
	// 	fmt.Printf("Total Memory: %v MB\n", vm.Total/1024/1024)
	// 	fmt.Printf("Used Memory: %v MB (%.2f%%)\n", vm.Used/1024/1024, vm.UsedPercent)
	// }

	// Channel to signal goroutine to stop (optional, for graceful shutdown)
	done := make(chan struct{})
	// Define a struct to hold CPU and memory info
	type CPUAndMem struct {
		CPUPercent        float64
		MemoryUsage       int64
		SystemCPUPercent  float64
		SystemMemoryUsage int64
	}
	// Create a channel to send CPU and memory info
	cpuMemChan := make(chan CPUAndMem, 1)

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				c, _ := p.CPUPercent()
				m, _ := p.MemoryInfo()
				vm, _ := mem.VirtualMemory()
				percent, _ := cpu.Percent(0, false)
				cpuMemChan <- CPUAndMem{CPUPercent: c, MemoryUsage: int64(m.RSS / 1024 / 1024), SystemCPUPercent: percent[0], SystemMemoryUsage: int64(vm.Used / 1024 / 1024)}
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()

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
	frameId := int64(0)
	buf := make([]byte, *bufferSize) // Buffer size for incoming messages

	for {
		n, _, err := listener.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Read error:", err)
			continue
		}

		var msg DataChannelMessage
		err = json.Unmarshal(buf[:n], &msg)
		// fmt.Println("Received message:", buf)
		// fmt.Printf("Received message:%#v \n", msg)
		if err != nil {
			fmt.Println("JSON unmarshal error:", err)
			continue
		}
		msg.MessageSentTimeVM1 = time.Now().UnixMicro()
		msg.PacketLossVM1 = int64(msg.FrameID) - frameId - 1 // Calculate packet loss
		frameId = int64(msg.FrameID)

		// Get latest CPU and memory info from the channel if available
		select {
		case cpuMem := <-cpuMemChan:
			msg.CPUPercentVM1 = cpuMem.CPUPercent   // Convert to percentage
			msg.MemoryUsageVM1 = cpuMem.MemoryUsage // Memory usage in MB
			msg.SystemCPUPercentVM1 = cpuMem.SystemCPUPercent
			msg.SystemMemoryUsageVM1 = cpuMem.SystemMemoryUsage // Memory usage in MB
			// fmt.Println("CPU and memory info for VM1:", msg.CPUPercentVM1, msg.MemoryUsageVM1)

		default:
			// No new CPU/mem info available, set to 0
			msg.CPUPercentVM1 = 0
			msg.MemoryUsageVM1 = 0
			msg.SystemCPUPercentVM1 = 0
			msg.SystemMemoryUsageVM1 = 0
		}
		data, err := json.Marshal(msg)
		if err != nil {
			fmt.Println("Error marshaling message:", err)
			continue
		}
		// if err := json.NewEncoder(&buf).Encode(msg); err != nil {
		// 	fmt.Println("Failed to encode payload:", err)
		// 	os.Exit(1)
		// }

		// fmt.Printf("Received %d bytes from %s, forwarding to %s\n", n, clientAddr, faddr)
		_, err = forwardConn.Write(data)
		if err != nil {
			fmt.Println("Forward error:", err)
		}
	}
}
