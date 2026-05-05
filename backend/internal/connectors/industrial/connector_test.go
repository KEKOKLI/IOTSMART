package industrial

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOPCUATCPRequestFromEndpointURL(t *testing.T) {
	request, err := (OPCUAProbeRequest{EndpointURL: "opc.tcp://plc.local:4841/UA/Server"}).opcuaTCPRequest()
	if err != nil {
		t.Fatalf("parse opcua endpoint: %v", err)
	}
	if request.Host != "plc.local" || request.Port != 4841 {
		t.Fatalf("unexpected tcp request: %#v", request)
	}
}

func TestOPCUATCPRequestDefaultPort(t *testing.T) {
	request, err := (OPCUAProbeRequest{Host: "127.0.0.1"}).opcuaTCPRequest()
	if err != nil {
		t.Fatalf("build opcua request: %v", err)
	}
	if request.Port != defaultOPCUAPort {
		t.Fatalf("unexpected default port: %d", request.Port)
	}
}

func TestODBCRequiresDSNOrConnectionString(t *testing.T) {
	if _, _, err := buildODBCConnectionString("", "", ""); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestODBCConnectionStringFromDSN(t *testing.T) {
	connectionString, endpoint, err := buildODBCConnectionString("FactoryHistorian", "", "")
	if err != nil {
		t.Fatalf("build odbc connection string: %v", err)
	}
	if connectionString != "DSN=FactoryHistorian;" || endpoint != "FactoryHistorian" {
		t.Fatalf("unexpected connection string %q endpoint %q", connectionString, endpoint)
	}
}

func TestBACnetReadPropertyFrame(t *testing.T) {
	frame, err := bacnetReadPropertyFrame(7, 0, 1, 85, nil)
	if err != nil {
		t.Fatalf("build bacnet read property frame: %v", err)
	}
	if frame[0] != 0x81 || frame[1] != 0x0a {
		t.Fatalf("unexpected bvlc header: %#v", frame[:2])
	}
	if got := int(binary.BigEndian.Uint16(frame[2:4])); got != len(frame) {
		t.Fatalf("unexpected frame length: got %d want %d", got, len(frame))
	}
	if frame[6] != 0x00 || frame[9] != 0x0c {
		t.Fatalf("unexpected apdu: %#v", frame[6:10])
	}
}

func TestBACnetApplicationValueReal(t *testing.T) {
	raw := []byte{0x44, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(raw[1:5], math.Float32bits(23.5))
	value, valueType, err := parseBACnetApplicationValue(raw)
	if err != nil {
		t.Fatalf("parse bacnet value: %v", err)
	}
	if valueType != "real" || value.(float32) != 23.5 {
		t.Fatalf("unexpected value: %v (%s)", value, valueType)
	}
}

func TestModbusRTUReadFrameCRC(t *testing.T) {
	frame := modbusRTUReadFrame(1, 3, 0, 2)
	if !validModbusCRC(frame) {
		t.Fatalf("expected valid crc")
	}
	if got := binary.LittleEndian.Uint16(frame[6:8]); got != 0x0bc4 {
		t.Fatalf("unexpected crc: 0x%04x", got)
	}
}

func TestParseModbusRTUReadResponse(t *testing.T) {
	response := []byte{1, 3, 4, 0, 10, 0, 20, 0, 0}
	binary.LittleEndian.PutUint16(response[7:9], modbusCRC(response[:7]))
	registers, data, err := parseModbusRTUReadResponse(response, 1, 3, 2)
	if err != nil {
		t.Fatalf("parse modbus rtu response: %v", err)
	}
	if registers[0] != 10 || registers[1] != 20 || len(data) != 4 {
		t.Fatalf("unexpected registers=%#v data=%#v", registers, data)
	}
}
