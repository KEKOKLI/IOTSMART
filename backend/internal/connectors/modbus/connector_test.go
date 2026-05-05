package modbus

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func TestReadRegistersWithFakeTCPServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake modbus server: %v", err)
	}
	defer listener.Close()

	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()

		request := make([]byte, 12)
		if _, err := io.ReadFull(conn, request); err != nil {
			done <- err
			return
		}

		response := make([]byte, 13)
		copy(response[0:2], request[0:2])
		binary.BigEndian.PutUint16(response[2:4], 0)
		binary.BigEndian.PutUint16(response[4:6], 7)
		response[6] = request[6]
		response[7] = request[7]
		response[8] = 4
		binary.BigEndian.PutUint16(response[9:11], 10)
		binary.BigEndian.PutUint16(response[11:13], 20)
		_, err = conn.Write(response)
		done <- err
	}()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split listener address: %v", err)
	}
	portNumber, err := net.LookupPort("tcp", port)
	if err != nil {
		t.Fatalf("parse listener port: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := ReadRegisters(ctx, ReadRequest{
		TCPRequest: TCPRequest{
			Host:      host,
			Port:      portNumber,
			UnitID:    1,
			TimeoutMS: 1000,
		},
		Function: "holding_registers",
		Address:  0,
		Quantity: 2,
	})
	if err != nil {
		t.Fatalf("read registers: %v", err)
	}
	if result.Registers[0] != 10 || result.Registers[1] != 20 {
		t.Fatalf("unexpected registers: %#v", result.Registers)
	}
	if err := <-done; err != nil {
		t.Fatalf("fake server error: %v", err)
	}
}
