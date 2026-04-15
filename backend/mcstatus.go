package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type MCStatus struct {
	PlayersOnline int
	PlayersMax    int
	MOTD          string
	Version       string
	Favicon       string
	LatencyMS     int64
}

type mcStatusResponse struct {
	Version struct {
		Name string `json:"name"`
	} `json:"version"`
	Players struct {
		Max    int `json:"max"`
		Online int `json:"online"`
	} `json:"players"`
	Description any    `json:"description"`
	Favicon     string `json:"favicon"`
}

func QueryMinecraftStatus(ctx context.Context, address string) (MCStatus, error) {
	start := time.Now()
	status := MCStatus{}

	host, port, err := splitAddress(address)
	if err != nil {
		return status, err
	}
	dialHost, dialPort := resolveMinecraftEndpoint(ctx, host, port)
	dialAddress := net.JoinHostPort(dialHost, strconv.Itoa(int(dialPort)))

	dialer := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", dialAddress)
	if err != nil {
		return status, fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	handshake := make([]byte, 0, 128)
	handshake = append(handshake, encodeVarInt(0)...)    // packet id
	handshake = append(handshake, encodeVarInt(47)...)   // protocol version
	handshake = append(handshake, encodeString(host)...) // server address
	handshake = binary.BigEndian.AppendUint16(handshake, dialPort)
	handshake = append(handshake, encodeVarInt(1)...) // next state = status

	if err := writePacket(conn, handshake); err != nil {
		return status, fmt.Errorf("write handshake: %w", err)
	}
	if err := writePacket(conn, encodeVarInt(0)); err != nil { // status request packet id
		return status, fmt.Errorf("write status request: %w", err)
	}

	payload, err := readPacket(conn)
	if err != nil {
		return status, fmt.Errorf("read status response: %w", err)
	}
	reader := bytesReader(payload)
	packetID, err := readVarInt(reader)
	if err != nil {
		return status, fmt.Errorf("read packet id: %w", err)
	}
	if packetID != 0 {
		return status, fmt.Errorf("unexpected packet id: %d", packetID)
	}
	jsonString, err := readString(reader)
	if err != nil {
		return status, fmt.Errorf("read payload string: %w", err)
	}

	var parsed mcStatusResponse
	if err := json.Unmarshal([]byte(jsonString), &parsed); err != nil {
		return status, fmt.Errorf("parse status json: %w", err)
	}

	status.PlayersOnline = parsed.Players.Online
	status.PlayersMax = parsed.Players.Max
	status.Version = parsed.Version.Name
	status.MOTD = parseMOTD(parsed.Description)
	status.Favicon = parsed.Favicon
	status.LatencyMS = time.Since(start).Milliseconds()

	return status, nil
}

func parseMOTD(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		return flattenMOTD(t)
	}
	return ""
}

func flattenMOTD(node map[string]any) string {
	text := ""
	if t, ok := node["text"].(string); ok {
		text += t
	}
	if extra, ok := node["extra"].([]any); ok {
		for _, child := range extra {
			switch c := child.(type) {
			case string:
				text += c
			case map[string]any:
				text += flattenMOTD(c)
			}
		}
	}
	return strings.TrimSpace(text)
}

func splitAddress(address string) (string, uint16, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, fmt.Errorf("invalid address %q: %w", address, err)
	}
	if host == "" {
		return "", 0, fmt.Errorf("address %q missing host", address)
	}
	portI, err := net.LookupPort("tcp", portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port %q: %w", portStr, err)
	}
	return host, uint16(portI), nil
}

func resolveMinecraftEndpoint(ctx context.Context, host string, port uint16) (string, uint16) {
	if port != 25565 || net.ParseIP(host) != nil {
		return host, port
	}

	_, records, err := net.DefaultResolver.LookupSRV(ctx, "minecraft", "tcp", host)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			return host, port
		}
		return host, port
	}
	if len(records) == 0 {
		return host, port
	}

	best := records[0]
	for _, record := range records[1:] {
		if record.Priority < best.Priority || (record.Priority == best.Priority && record.Weight > best.Weight) {
			best = record
		}
	}

	target := strings.TrimSuffix(best.Target, ".")
	if target == "" {
		return host, port
	}
	return target, best.Port
}

func encodeVarInt(value int) []byte {
	out := make([]byte, 0, 5)
	for {
		temp := byte(value & 0x7F)
		value >>= 7
		if value != 0 {
			temp |= 0x80
		}
		out = append(out, temp)
		if value == 0 {
			return out
		}
	}
}

func readVarInt(r io.ByteReader) (int, error) {
	var numRead int
	var result int
	for {
		if numRead >= 5 {
			return 0, fmt.Errorf("varint too large")
		}
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		value := int(b & 0x7F)
		result |= value << (7 * numRead)
		numRead++
		if (b & 0x80) == 0 {
			return result, nil
		}
	}
}

func encodeString(value string) []byte {
	encoded := []byte(value)
	out := make([]byte, 0, len(encoded)+5)
	out = append(out, encodeVarInt(len(encoded))...)
	out = append(out, encoded...)
	return out
}

func readString(r io.ByteReader) (string, error) {
	length, err := readVarInt(r)
	if err != nil {
		return "", err
	}
	if length < 0 || length > 1<<20 {
		return "", fmt.Errorf("invalid string length: %d", length)
	}
	buf := make([]byte, length)
	for i := 0; i < length; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		buf[i] = b
	}
	return string(buf), nil
}

func writePacket(w io.Writer, payload []byte) error {
	packet := make([]byte, 0, len(payload)+5)
	packet = append(packet, encodeVarInt(len(payload))...)
	packet = append(packet, payload...)
	_, err := w.Write(packet)
	return err
}

func readPacket(r io.Reader) ([]byte, error) {
	br := bufio.NewReader(r)
	length, err := readVarInt(br)
	if err != nil {
		return nil, err
	}
	if length <= 0 || length > 1<<20 {
		return nil, fmt.Errorf("invalid packet length: %d", length)
	}
	payload := make([]byte, length)
	_, err = io.ReadFull(br, payload)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func bytesReader(b []byte) *bufio.Reader {
	return bufio.NewReaderSize(bytes.NewReader(b), len(b))
}
