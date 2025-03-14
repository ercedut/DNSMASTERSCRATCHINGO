package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

const (
	DNS_PORT    = 53
	BUFFER_SIZE = 512
)

type DNSHeader struct {
	ID      uint16
	Flags   uint16
	Qdcount uint16
	Ancount uint16
	Nscount uint16
	Arcount uint16
}

type DNSQuestion struct {
	Qname  string
	Qtype  uint16
	Qclass uint16
}

type DNSRecord struct {
	Name  string
	Type  uint16
	Class uint16
	TTL   uint32
	Data  string
}

func main() {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", DNS_PORT))
	if err != nil {
		log.Fatalf("Failed to resolve UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatalf("Failed to listen on UDP: %v", err)
	}
	defer conn.Close()

	log.Printf("DNS server started on port %d\n", DNS_PORT)

	for {
		handleRequest(conn)
	}
}

func handleRequest(conn *net.UDPConn) {
	buffer := make([]byte, BUFFER_SIZE)
	n, addr, err := conn.ReadFromUDP(buffer)
	if err != nil {
		log.Printf("Failed to read from UDP: %v", err)
		return
	}

	header := parseHeader(buffer[:12])
	questions := parseQuestions(buffer[12:n], header.Qdcount)

	log.Printf("Received DNS query from %s: %+v\n", addr, questions)

	response := buildResponse(header, questions)
	_, err = conn.WriteToUDP(response, addr)
	if err != nil {
		log.Printf("Failed to write response to UDP: %v", err)
	}
}

func parseHeader(data []byte) DNSHeader {
	return DNSHeader{
		ID:      binary.BigEndian.Uint16(data[0:2]),
		Flags:   binary.BigEndian.Uint16(data[2:4]),
		Qdcount: binary.BigEndian.Uint16(data[4:6]),
		Ancount: binary.BigEndian.Uint16(data[6:8]),
		Nscount: binary.BigEndian.Uint16(data[8:10]),
		Arcount: binary.BigEndian.Uint16(data[10:12]),
	}
}

func parseQuestions(data []byte, qdcount uint16) []DNSQuestion {
	var questions []DNSQuestion
	offset := 0

	for i := 0; i < int(qdcount); i++ {
		qname, newOffset := parseQname(data, offset)
		offset = newOffset

		qtype := binary.BigEndian.Uint16(data[offset : offset+2])
		qclass := binary.BigEndian.Uint16(data[offset+2 : offset+4])
		offset += 4

		questions = append(questions, DNSQuestion{
			Qname:  qname,
			Qtype:  qtype,
			Qclass: qclass,
		})
	}

	return questions
}

func parseQname(data []byte, offset int) (string, int) {
	var qname string
	length := int(data[offset])
	originalOffset := offset

	for length != 0 {
		if len(qname) > 0 {
			qname += "."
		}
		offset++
		qname += string(data[offset : offset+length])
		offset += length
		length = int(data[offset])
	}

	return qname, originalOffset + len(qname) + 2
}

func buildResponse(header DNSHeader, questions []DNSQuestion) []byte {
	response := make([]byte, 0)

	// Header
	response = append(response, byte(header.ID>>8), byte(header.ID))
	response = append(response, byte(0x80), byte(0x00)) // Flags: Standard query response
	response = append(response, byte(header.Qdcount>>8), byte(header.Qdcount))
	response = append(response, byte(1>>8), byte(1)) // Answer count
	response = append(response, byte(0>>8), byte(0)) // Authority count
	response = append(response, byte(0>>8), byte(0)) // Additional count

	// Questions
	for _, question := range questions {
		response = append(response, encodeQname(question.Qname)...)
		response = append(response, byte(question.Qtype>>8), byte(question.Qtype))
		response = append(response, byte(question.Qclass>>8), byte(question.Qclass))
	}

	// Answer
	response = append(response, encodeQname(questions[0].Qname)...)
	response = append(response, byte(questions[0].Qtype>>8), byte(questions[0].Qtype))
	response = append(response, byte(questions[0].Qclass>>8), byte(questions[0].Qclass))
	response = append(response, byte(0x00), byte(0x00), byte(0x00), byte(0x3C)) // TTL: 60 seconds
	response = append(response, byte(0x00), byte(0x04))                         // Data length: 4 bytes
	response = append(response, byte(127), byte(0), byte(0), byte(1))           // IP: 127.0.0.1

	return response
}

func encodeQname(qname string) []byte {
	var encoded []byte
	parts := splitDomain(qname)

	for _, part := range parts {
		encoded = append(encoded, byte(len(part)))
		encoded = append(encoded, []byte(part)...)
	}

	encoded = append(encoded, byte(0x00))
	return encoded
}

func splitDomain(domain string) []string {
	var parts []string
	start := 0

	for i, char := range domain {
		if char == '.' {
			parts = append(parts, domain[start:i])
			start = i + 1
		}
	}

	if start < len(domain) {
		parts = append(parts, domain[start:])
	}

	return parts
}
