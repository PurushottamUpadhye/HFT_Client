package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"time"
)

type Packet struct {
	Symbol         string
	BuySell        string
	Quantity       int32
	Price          int32
	PacketSequence int32
}

// Change the ip and port details according to server configuration.
var (
	IP          = "192.168.0.148"
	PORT        = "3000"
	PACKET_SIZE = 17
)

func Start() {

	conn, err := createConnection()
	if err != nil {
		fmt.Println("Error occured while initiating connection:", err)
		return
	}

	// Request server to stream all packets
	received_ticks, err := sendStreamAllPackets(conn)
	if err != nil {
		fmt.Println("Error occured while receiving ticks from server.")
		return
	}

	// Wait for the server to send the packets and close the connection
	time.Sleep(2 * time.Second)

	// Establish connection with server to get missed packets
	conn, err = createConnection()
	if err != nil {
		fmt.Println("Error occured while initiating connection:", err)
		return
	}

	// Finds the missing seq num
	missed_seq := findMissingSeq(received_ticks)
	fmt.Println("Missed seq number:", missed_seq, len(missed_seq))

	// Request server to send missed packets
	err = resendPacket(conn, missed_seq)
	if err != nil {
		fmt.Println("Error in resend packet:", err)
	}

}

func createConnection() (net.Conn, error) {

	address := net.JoinHostPort(IP, PORT)

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	return conn, nil
}

func sendStreamAllPackets(conn net.Conn) ([]Packet, error) {

	header := []byte{1, 0}

	// Write the header to the connection
	_, err := conn.Write(header)
	if err != nil {
		fmt.Println("Error sending stream request:", err)
		return nil, err
	}

	ticks := receiveData(conn)

	return ticks, nil
}

func receiveData(conn net.Conn) []Packet {

	buffer := make([]byte, 2048)
	var packets []Packet
	for {
		// Read from the connection
		n, err := conn.Read(buffer)
		if err != nil {
			if err.Error() == "EOF" {
				break // Connection closed by server
			} else {
				fmt.Println("Error reading data:", err)
				break
			}
		}

		for i := 0; i < n; i += PACKET_SIZE {
			if i+PACKET_SIZE <= n {
				packet := buffer[i : i+PACKET_SIZE]

				parsedpacket, err := parseResponse(packet)
				if err != nil {
					fmt.Println("Error in parsing response:", err)
				}

				packets = append(packets, parsedpacket)

			} else {
				fmt.Println("Received an incomplete packet")
			}
		}
	}

	fmt.Println("ALL RECEIVED TICKS:", packets)

	return packets
}

func parseResponse(response []byte) (Packet, error) {

	var packet Packet

	// Loop over the response data in chunks of packet size
	for i := 0; i < len(response); i += PACKET_SIZE {
		// Ensure we don't go beyond the length of the response
		if i+PACKET_SIZE > len(response) {
			return packet, fmt.Errorf("response data is incomplete")
		}

		packetData := response[i : i+PACKET_SIZE]

		packet.Symbol = string(packetData[:4])                                    // First 4 bytes are the symbol.
		packet.BuySell = string(packetData[4:5])                                  // 5th byte is the Buy/Sell indicator.
		packet.Quantity = int32(binary.BigEndian.Uint32(packetData[5:9]))         // Next 4 bytes are quantity.
		packet.Price = int32(binary.BigEndian.Uint32(packetData[9:13]))           // Next 4 bytes are price.
		packet.PacketSequence = int32(binary.BigEndian.Uint32(packetData[13:17])) // Last 4 bytes are packet sequence.

	}

	return packet, nil
}

func findMissingSeq(data []Packet) []int32 {
	sequences := make([]int32, len(data))
	for i, packet := range data {
		sequences[i] = packet.PacketSequence
	}

	// Sort the sequence numbers
	sort.Slice(sequences, func(i, j int) bool {
		return sequences[i] < sequences[j]
	})

	// Find missing sequence numbers
	var missing []int32

	// Initial seq num can be missed but final seq num is not missed
	start_seq := int32(1)
	for i := start_seq; i <= sequences[len(sequences)-1]; i++ {
		if !contains(sequences, i) {
			missing = append(missing, i)
		}
	}

	return missing
}

// Helper function to check if a slice contains a value
func contains(slice []int32, value int32) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func resendPacket(conn net.Conn, resendSeqs []int32) error {

	var packets []Packet
	buffer := make([]byte, 2048)

	for _, seq := range resendSeqs {

		header := []byte{2, uint8(seq)}

		// Write the header to the connection
		_, err := conn.Write(header)
		if err != nil {
			fmt.Println("Error sending stream request:", err)
			return err
		}

		n, err := conn.Read(buffer)
		if err != nil {
			if err.Error() == "EOF" {
				break // Connection closed by server
			} else {
				fmt.Println("Error reading data:", err)
				break
			}
		}

		for i := 0; i < n; i += PACKET_SIZE {
			if i+PACKET_SIZE <= n {
				packet := buffer[i : i+PACKET_SIZE]

				parsedpacket, err := parseResponse(packet)
				if err != nil {
					fmt.Println("Error in parsing response:", err)
				}

				packets = append(packets, parsedpacket)

			} else {
				fmt.Println("Received an incomplete packet")
			}
		}
	}

	fmt.Println("RECEIVED MISSED PACKETS:", packets)

	return nil
}
