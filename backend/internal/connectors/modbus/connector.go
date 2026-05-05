package modbus

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTCPPort   = 502
	defaultTimeoutMS = 3000
	maxRegisterCount = 125
)

type Connector struct{}

type TCPRequest struct {
	Host      string `json:"host"`
	Port      int    `json:"port,omitempty"`
	UnitID    uint8  `json:"unit_id,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type ReadRequest struct {
	TCPRequest
	Function     string `json:"function,omitempty"`
	FunctionCode uint8  `json:"function_code,omitempty"`
	Address      uint16 `json:"address"`
	Quantity     uint16 `json:"quantity"`
}

type TestResult struct {
	OK         bool   `json:"ok"`
	Endpoint   string `json:"endpoint"`
	DurationMS int64  `json:"duration_ms"`
}

type ReadResult struct {
	OK           bool     `json:"ok"`
	Endpoint     string   `json:"endpoint"`
	UnitID       uint8    `json:"unit_id"`
	FunctionCode uint8    `json:"function_code"`
	Address      uint16   `json:"address"`
	Quantity     uint16   `json:"quantity"`
	Registers    []uint16 `json:"registers"`
	Bytes        []int    `json:"bytes"`
	DurationMS   int64    `json:"duration_ms"`
}

func (Connector) Name() string {
	return "modbus"
}

func (Connector) Status() string {
	return "available"
}

func TestTCPConnection(ctx context.Context, request TCPRequest) (TestResult, error) {
	normalized, err := normalizeTCPRequest(request)
	if err != nil {
		return TestResult{}, err
	}

	started := time.Now()
	conn, err := dialTCP(ctx, normalized)
	if err != nil {
		return TestResult{}, err
	}
	_ = conn.Close()

	return TestResult{
		OK:         true,
		Endpoint:   endpoint(normalized),
		DurationMS: time.Since(started).Milliseconds(),
	}, nil
}

func ReadRegisters(ctx context.Context, request ReadRequest) (ReadResult, error) {
	normalized, err := normalizeReadRequest(request)
	if err != nil {
		return ReadResult{}, err
	}

	started := time.Now()
	conn, err := dialTCP(ctx, normalized.TCPRequest)
	if err != nil {
		return ReadResult{}, err
	}
	defer conn.Close()

	timeout := timeoutDuration(normalized.TimeoutMS)
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return ReadResult{}, err
	}

	transactionID := uint16(time.Now().UnixNano() % math.MaxUint16)
	requestFrame := make([]byte, 12)
	binary.BigEndian.PutUint16(requestFrame[0:2], transactionID)
	binary.BigEndian.PutUint16(requestFrame[2:4], 0)
	binary.BigEndian.PutUint16(requestFrame[4:6], 6)
	requestFrame[6] = normalized.UnitID
	requestFrame[7] = normalized.FunctionCode
	binary.BigEndian.PutUint16(requestFrame[8:10], normalized.Address)
	binary.BigEndian.PutUint16(requestFrame[10:12], normalized.Quantity)

	if _, err := conn.Write(requestFrame); err != nil {
		return ReadResult{}, fmt.Errorf("write modbus request: %w", err)
	}

	header := make([]byte, 7)
	if _, err := io.ReadFull(conn, header); err != nil {
		return ReadResult{}, fmt.Errorf("read modbus header: %w", err)
	}
	if got := binary.BigEndian.Uint16(header[0:2]); got != transactionID {
		return ReadResult{}, fmt.Errorf("unexpected transaction id: got %d want %d", got, transactionID)
	}
	if got := binary.BigEndian.Uint16(header[2:4]); got != 0 {
		return ReadResult{}, fmt.Errorf("unexpected protocol id: %d", got)
	}
	if header[6] != normalized.UnitID {
		return ReadResult{}, fmt.Errorf("unexpected unit id: got %d want %d", header[6], normalized.UnitID)
	}

	length := binary.BigEndian.Uint16(header[4:6])
	if length < 2 {
		return ReadResult{}, fmt.Errorf("invalid modbus response length: %d", length)
	}
	pdu := make([]byte, int(length)-1)
	if _, err := io.ReadFull(conn, pdu); err != nil {
		return ReadResult{}, fmt.Errorf("read modbus pdu: %w", err)
	}
	if len(pdu) < 2 {
		return ReadResult{}, fmt.Errorf("short modbus pdu")
	}
	if pdu[0]&0x80 != 0 {
		return ReadResult{}, fmt.Errorf("modbus exception code %d", pdu[1])
	}
	if pdu[0] != normalized.FunctionCode {
		return ReadResult{}, fmt.Errorf("unexpected function code: got %d want %d", pdu[0], normalized.FunctionCode)
	}

	byteCount := int(pdu[1])
	expectedByteCount := int(normalized.Quantity) * 2
	if byteCount != expectedByteCount || len(pdu[2:]) < byteCount {
		return ReadResult{}, fmt.Errorf("unexpected byte count: got %d want %d", byteCount, expectedByteCount)
	}

	data := pdu[2 : 2+byteCount]
	registers := make([]uint16, 0, normalized.Quantity)
	for index := 0; index < byteCount; index += 2 {
		registers = append(registers, binary.BigEndian.Uint16(data[index:index+2]))
	}

	return ReadResult{
		OK:           true,
		Endpoint:     endpoint(normalized.TCPRequest),
		UnitID:       normalized.UnitID,
		FunctionCode: normalized.FunctionCode,
		Address:      normalized.Address,
		Quantity:     normalized.Quantity,
		Registers:    registers,
		Bytes:        byteInts(data),
		DurationMS:   time.Since(started).Milliseconds(),
	}, nil
}

func normalizeReadRequest(request ReadRequest) (ReadRequest, error) {
	tcpRequest, err := normalizeTCPRequest(request.TCPRequest)
	if err != nil {
		return ReadRequest{}, err
	}
	request.TCPRequest = tcpRequest
	if request.Quantity == 0 || request.Quantity > maxRegisterCount {
		return ReadRequest{}, fmt.Errorf("quantity must be between 1 and %d", maxRegisterCount)
	}

	functionCode := request.FunctionCode
	if functionCode == 0 {
		functionCode, err = parseFunctionCode(request.Function)
		if err != nil {
			return ReadRequest{}, err
		}
	}
	if functionCode != 3 && functionCode != 4 {
		return ReadRequest{}, fmt.Errorf("only function codes 3 and 4 are supported")
	}
	request.FunctionCode = functionCode
	return request, nil
}

func normalizeTCPRequest(request TCPRequest) (TCPRequest, error) {
	request.Host = strings.TrimSpace(request.Host)
	if request.Host == "" {
		return TCPRequest{}, fmt.Errorf("host is required")
	}
	if request.Port == 0 {
		request.Port = defaultTCPPort
	}
	if request.Port < 1 || request.Port > 65535 {
		return TCPRequest{}, fmt.Errorf("port must be between 1 and 65535")
	}
	if request.UnitID == 0 {
		request.UnitID = 1
	}
	if request.TimeoutMS <= 0 {
		request.TimeoutMS = defaultTimeoutMS
	}
	return request, nil
}

func dialTCP(ctx context.Context, request TCPRequest) (net.Conn, error) {
	dialer := net.Dialer{Timeout: timeoutDuration(request.TimeoutMS)}
	conn, err := dialer.DialContext(ctx, "tcp", endpoint(request))
	if err != nil {
		return nil, fmt.Errorf("connect modbus tcp: %w", err)
	}
	return conn, nil
}

func parseFunctionCode(value string) (uint8, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "holding", "holding_register", "holding_registers", "read_holding_registers", "3", "03":
		return 3, nil
	case "input", "input_register", "input_registers", "read_input_registers", "4", "04":
		return 4, nil
	default:
		return 0, fmt.Errorf("unsupported function: %s", value)
	}
}

func timeoutDuration(timeoutMS int) time.Duration {
	if timeoutMS <= 0 {
		timeoutMS = defaultTimeoutMS
	}
	return time.Duration(timeoutMS) * time.Millisecond
}

func endpoint(request TCPRequest) string {
	return net.JoinHostPort(request.Host, strconv.Itoa(request.Port))
}

func byteInts(data []byte) []int {
	result := make([]int, 0, len(data))
	for _, value := range data {
		result = append(result, int(value))
	}
	return result
}
