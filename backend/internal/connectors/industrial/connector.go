package industrial

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	ads "github.com/RuneRoven/go-ads"
	_ "github.com/alexbrainman/odbc"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
	serial "go.bug.st/serial"
)

const (
	defaultTimeoutMS       = 3000
	defaultOPCUAPort       = 4840
	defaultBACnetPort      = 47808
	defaultADSPort         = 48898
	defaultADSAMSPort      = 851
	defaultADSLocalPort    = 10500
	defaultODBCMaxRows     = 100
	maxModbusRTURegisters  = 125
	maxODBCRows            = 1000
	defaultOPCUABrowseNode = "ns=0;i=85"
)

type TCPProbeRequest struct {
	Host      string `json:"host"`
	Port      int    `json:"port,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type OPCUAProbeRequest struct {
	EndpointURL string `json:"endpoint_url,omitempty"`
	Host        string `json:"host,omitempty"`
	Port        int    `json:"port,omitempty"`
	TimeoutMS   int    `json:"timeout_ms,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
}

type OPCUABrowseRequest struct {
	EndpointURL     string `json:"endpoint_url,omitempty"`
	Host            string `json:"host,omitempty"`
	Port            int    `json:"port,omitempty"`
	TimeoutMS       int    `json:"timeout_ms,omitempty"`
	Username        string `json:"username,omitempty"`
	Password        string `json:"password,omitempty"`
	NodeID          string `json:"node_id,omitempty"`
	ReferenceTypeID string `json:"reference_type_id,omitempty"`
	MaxReferences   uint32 `json:"max_references,omitempty"`
	NodeClassMask   uint32 `json:"node_class_mask,omitempty"`
	IncludeSubtypes bool   `json:"include_subtypes,omitempty"`
}

type OPCUAReadRequest struct {
	EndpointURL string `json:"endpoint_url,omitempty"`
	Host        string `json:"host,omitempty"`
	Port        int    `json:"port,omitempty"`
	TimeoutMS   int    `json:"timeout_ms,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	NodeID      string `json:"node_id"`
	Attribute   string `json:"attribute,omitempty"`
	AttributeID uint32 `json:"attribute_id,omitempty"`
	MaxAgeMS    int    `json:"max_age_ms,omitempty"`
}

type BACnetProbeRequest struct {
	Host           string `json:"host"`
	Port           int    `json:"port,omitempty"`
	TimeoutMS      int    `json:"timeout_ms,omitempty"`
	ExpectResponse bool   `json:"expect_response,omitempty"`
}

type BACnetReadPropertyRequest struct {
	Host           string  `json:"host"`
	Port           int     `json:"port,omitempty"`
	TimeoutMS      int     `json:"timeout_ms,omitempty"`
	ObjectType     string  `json:"object_type,omitempty"`
	ObjectTypeID   uint16  `json:"object_type_id,omitempty"`
	ObjectInstance uint32  `json:"object_instance"`
	Property       string  `json:"property,omitempty"`
	PropertyID     uint32  `json:"property_id,omitempty"`
	ArrayIndex     *uint32 `json:"array_index,omitempty"`
}

type ModbusRTUProbeRequest struct {
	Port      string `json:"port"`
	BaudRate  int    `json:"baud_rate,omitempty"`
	DataBits  int    `json:"data_bits,omitempty"`
	Parity    string `json:"parity,omitempty"`
	StopBits  int    `json:"stop_bits,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type ModbusRTUReadRequest struct {
	Port         string `json:"port"`
	BaudRate     int    `json:"baud_rate,omitempty"`
	DataBits     int    `json:"data_bits,omitempty"`
	Parity       string `json:"parity,omitempty"`
	StopBits     int    `json:"stop_bits,omitempty"`
	TimeoutMS    int    `json:"timeout_ms,omitempty"`
	UnitID       uint8  `json:"unit_id,omitempty"`
	Function     string `json:"function,omitempty"`
	FunctionCode uint8  `json:"function_code,omitempty"`
	Address      uint16 `json:"address"`
	Quantity     uint16 `json:"quantity"`
}

type ADSReadSymbolRequest struct {
	Host       string `json:"host"`
	Port       int    `json:"port,omitempty"`
	NetID      string `json:"net_id,omitempty"`
	AMSPort    int    `json:"ams_port,omitempty"`
	LocalNetID string `json:"local_net_id,omitempty"`
	LocalPort  int    `json:"local_port,omitempty"`
	Symbol     string `json:"symbol"`
	TimeoutMS  int    `json:"timeout_ms,omitempty"`
	Local      bool   `json:"local,omitempty"`
}

type ODBCProbeRequest struct {
	DSN              string `json:"dsn,omitempty"`
	Driver           string `json:"driver,omitempty"`
	ConnectionString string `json:"connection_string,omitempty"`
	TimeoutMS        int    `json:"timeout_ms,omitempty"`
}

type ODBCQueryRequest struct {
	DSN              string `json:"dsn,omitempty"`
	Driver           string `json:"driver,omitempty"`
	ConnectionString string `json:"connection_string,omitempty"`
	TimeoutMS        int    `json:"timeout_ms,omitempty"`
	Query            string `json:"query"`
	MaxRows          int    `json:"max_rows,omitempty"`
}

type ProbeResult struct {
	OK         bool   `json:"ok"`
	Protocol   string `json:"protocol"`
	Status     string `json:"status"`
	Endpoint   string `json:"endpoint,omitempty"`
	Message    string `json:"message,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

type OPCUAReference struct {
	NodeID          string `json:"node_id"`
	BrowseName      string `json:"browse_name"`
	DisplayName     string `json:"display_name"`
	NodeClass       string `json:"node_class"`
	ReferenceTypeID string `json:"reference_type_id"`
	TypeDefinition  string `json:"type_definition,omitempty"`
	IsForward       bool   `json:"is_forward"`
}

type OPCUABrowseResult struct {
	OK         bool             `json:"ok"`
	Protocol   string           `json:"protocol"`
	Endpoint   string           `json:"endpoint"`
	NodeID     string           `json:"node_id"`
	References []OPCUAReference `json:"references"`
	DurationMS int64            `json:"duration_ms"`
}

type OPCUAReadResult struct {
	OK              bool   `json:"ok"`
	Protocol        string `json:"protocol"`
	Endpoint        string `json:"endpoint"`
	NodeID          string `json:"node_id"`
	Attribute       string `json:"attribute"`
	Status          string `json:"status"`
	Value           any    `json:"value"`
	ValueType       string `json:"value_type,omitempty"`
	SourceTimestamp string `json:"source_timestamp,omitempty"`
	ServerTimestamp string `json:"server_timestamp,omitempty"`
	DurationMS      int64  `json:"duration_ms"`
}

type BACnetReadPropertyResult struct {
	OK             bool   `json:"ok"`
	Protocol       string `json:"protocol"`
	Endpoint       string `json:"endpoint"`
	ObjectTypeID   uint16 `json:"object_type_id"`
	ObjectInstance uint32 `json:"object_instance"`
	PropertyID     uint32 `json:"property_id"`
	Value          any    `json:"value"`
	ValueType      string `json:"value_type"`
	RawHex         string `json:"raw_hex"`
	DurationMS     int64  `json:"duration_ms"`
}

type ModbusRTUReadResult struct {
	OK           bool     `json:"ok"`
	Protocol     string   `json:"protocol"`
	Endpoint     string   `json:"endpoint"`
	UnitID       uint8    `json:"unit_id"`
	FunctionCode uint8    `json:"function_code"`
	Address      uint16   `json:"address"`
	Quantity     uint16   `json:"quantity"`
	Registers    []uint16 `json:"registers"`
	Bytes        []int    `json:"bytes"`
	DurationMS   int64    `json:"duration_ms"`
}

type ADSReadSymbolResult struct {
	OK         bool   `json:"ok"`
	Protocol   string `json:"protocol"`
	Endpoint   string `json:"endpoint"`
	NetID      string `json:"net_id"`
	AMSPort    int    `json:"ams_port"`
	Symbol     string `json:"symbol"`
	DataType   string `json:"data_type,omitempty"`
	Length     uint32 `json:"length,omitempty"`
	Value      string `json:"value"`
	DurationMS int64  `json:"duration_ms"`
}

type ODBCQueryResult struct {
	OK         bool             `json:"ok"`
	Protocol   string           `json:"protocol"`
	Endpoint   string           `json:"endpoint"`
	Columns    []string         `json:"columns"`
	Rows       []map[string]any `json:"rows"`
	RowCount   int              `json:"row_count"`
	DurationMS int64            `json:"duration_ms"`
}

func TestOPCUA(ctx context.Context, request OPCUAProbeRequest) (ProbeResult, error) {
	tcpRequest, err := request.opcuaTCPRequest()
	if err != nil {
		return ProbeResult{}, err
	}
	result, err := testTCP(ctx, "opcua", tcpRequest)
	if err != nil {
		return ProbeResult{}, err
	}
	result.Message = "TCP endpoint reachable. Use /api/v1/protocols/opcua/browse or /read for OPC UA session calls."
	return result, nil
}

func BrowseOPCUA(ctx context.Context, request OPCUABrowseRequest) (OPCUABrowseResult, error) {
	started := time.Now()
	client, endpoint, err := connectOPCUA(ctx, request.EndpointURL, request.Host, request.Port, request.TimeoutMS, request.Username, request.Password)
	if err != nil {
		return OPCUABrowseResult{}, err
	}
	defer client.Close(ctx)

	nodeIDText := strings.TrimSpace(request.NodeID)
	if nodeIDText == "" {
		nodeIDText = defaultOPCUABrowseNode
	}
	nodeID, err := ua.ParseNodeID(nodeIDText)
	if err != nil {
		return OPCUABrowseResult{}, fmt.Errorf("parse node_id: %w", err)
	}
	referenceTypeID := ua.NewNumericNodeID(0, id.References)
	if strings.TrimSpace(request.ReferenceTypeID) != "" {
		referenceTypeID, err = ua.ParseNodeID(request.ReferenceTypeID)
		if err != nil {
			return OPCUABrowseResult{}, fmt.Errorf("parse reference_type_id: %w", err)
		}
	}
	maxReferences := request.MaxReferences
	if maxReferences == 0 {
		maxReferences = 50
	}
	nodeClassMask := request.NodeClassMask
	if nodeClassMask == 0 {
		nodeClassMask = uint32(ua.NodeClassAll)
	}

	response, err := client.Browse(ctx, &ua.BrowseRequest{
		View:                          &ua.ViewDescription{},
		RequestedMaxReferencesPerNode: maxReferences,
		NodesToBrowse: []*ua.BrowseDescription{
			{
				NodeID:          nodeID,
				BrowseDirection: ua.BrowseDirectionForward,
				ReferenceTypeID: referenceTypeID,
				IncludeSubtypes: request.IncludeSubtypes,
				NodeClassMask:   nodeClassMask,
				ResultMask:      uint32(ua.BrowseResultMaskAll),
			},
		},
	})
	if err != nil {
		return OPCUABrowseResult{}, fmt.Errorf("opcua browse: %w", err)
	}
	if len(response.Results) == 0 {
		return OPCUABrowseResult{}, fmt.Errorf("opcua browse returned no results")
	}
	result := response.Results[0]
	if result.StatusCode != ua.StatusOK {
		return OPCUABrowseResult{}, fmt.Errorf("opcua browse status: %s", result.StatusCode)
	}

	references := make([]OPCUAReference, 0, len(result.References))
	for _, ref := range result.References {
		references = append(references, OPCUAReference{
			NodeID:          stringer(ref.NodeID),
			BrowseName:      qualifiedName(ref.BrowseName),
			DisplayName:     localizedText(ref.DisplayName),
			NodeClass:       ref.NodeClass.String(),
			ReferenceTypeID: stringer(ref.ReferenceTypeID),
			TypeDefinition:  stringer(ref.TypeDefinition),
			IsForward:       ref.IsForward,
		})
	}
	return OPCUABrowseResult{
		OK:         true,
		Protocol:   "opcua",
		Endpoint:   endpoint,
		NodeID:     nodeID.String(),
		References: references,
		DurationMS: time.Since(started).Milliseconds(),
	}, nil
}

func ReadOPCUANode(ctx context.Context, request OPCUAReadRequest) (OPCUAReadResult, error) {
	started := time.Now()
	client, endpoint, err := connectOPCUA(ctx, request.EndpointURL, request.Host, request.Port, request.TimeoutMS, request.Username, request.Password)
	if err != nil {
		return OPCUAReadResult{}, err
	}
	defer client.Close(ctx)

	nodeID, err := ua.ParseNodeID(strings.TrimSpace(request.NodeID))
	if err != nil {
		return OPCUAReadResult{}, fmt.Errorf("parse node_id: %w", err)
	}
	attributeID, attributeName, err := opcuaAttributeID(request.Attribute, request.AttributeID)
	if err != nil {
		return OPCUAReadResult{}, err
	}
	maxAge := request.MaxAgeMS
	if maxAge <= 0 {
		maxAge = 2000
	}
	response, err := client.Read(ctx, &ua.ReadRequest{
		MaxAge:             float64(maxAge),
		TimestampsToReturn: ua.TimestampsToReturnBoth,
		NodesToRead: []*ua.ReadValueID{
			{NodeID: nodeID, AttributeID: attributeID},
		},
	})
	if err != nil {
		return OPCUAReadResult{}, fmt.Errorf("opcua read: %w", err)
	}
	if len(response.Results) == 0 {
		return OPCUAReadResult{}, fmt.Errorf("opcua read returned no results")
	}
	dataValue := response.Results[0]
	if dataValue.Status != ua.StatusOK {
		return OPCUAReadResult{}, fmt.Errorf("opcua read status: %s", dataValue.Status)
	}

	value, valueType := opcuaValue(dataValue.Value)
	return OPCUAReadResult{
		OK:              true,
		Protocol:        "opcua",
		Endpoint:        endpoint,
		NodeID:          nodeID.String(),
		Attribute:       attributeName,
		Status:          fmt.Sprint(dataValue.Status),
		Value:           value,
		ValueType:       valueType,
		SourceTimestamp: timeOrEmpty(dataValue.SourceTimestamp),
		ServerTimestamp: timeOrEmpty(dataValue.ServerTimestamp),
		DurationMS:      time.Since(started).Milliseconds(),
	}, nil
}

func TestADS(ctx context.Context, request TCPProbeRequest) (ProbeResult, error) {
	if request.Port == 0 {
		request.Port = defaultADSPort
	}
	result, err := testTCP(ctx, "ads", request)
	if err != nil {
		return ProbeResult{}, err
	}
	result.Message = "ADS TCP port reachable. Use /api/v1/protocols/ads/read-symbol for TwinCAT symbol reads."
	return result, nil
}

func ReadADSSymbol(ctx context.Context, request ADSReadSymbolRequest) (ADSReadSymbolResult, error) {
	started := time.Now()
	request.Host = strings.TrimSpace(request.Host)
	request.Symbol = strings.TrimSpace(request.Symbol)
	if request.Host == "" {
		return ADSReadSymbolResult{}, fmt.Errorf("host is required")
	}
	if request.Symbol == "" {
		return ADSReadSymbolResult{}, fmt.Errorf("symbol is required")
	}
	if request.Port == 0 {
		request.Port = defaultADSPort
	}
	if request.AMSPort == 0 {
		request.AMSPort = defaultADSAMSPort
	}
	if request.LocalPort == 0 {
		request.LocalPort = defaultADSLocalPort
	}
	if request.LocalNetID == "" {
		request.LocalNetID = "auto"
	}
	if request.NetID == "" {
		request.NetID = netIDFromHost(request.Host)
	}
	if request.NetID == "" {
		return ADSReadSymbolResult{}, fmt.Errorf("net_id is required when host is not an IPv4 address")
	}

	conn, err := ads.NewConnection(ctx, request.Host, request.Port, request.NetID, request.AMSPort, request.LocalNetID, request.LocalPort, timeoutDuration(request.TimeoutMS))
	if err != nil {
		return ADSReadSymbolResult{}, err
	}
	if err := conn.Connect(request.Local); err != nil {
		return ADSReadSymbolResult{}, fmt.Errorf("connect ads: %w", err)
	}
	defer conn.Close()

	value, err := conn.ReadFromSymbol(request.Symbol)
	if err != nil {
		return ADSReadSymbolResult{}, fmt.Errorf("read ads symbol: %w", err)
	}
	symbol, _ := conn.GetSymbol(request.Symbol)

	result := ADSReadSymbolResult{
		OK:         true,
		Protocol:   "ads",
		Endpoint:   net.JoinHostPort(request.Host, strconv.Itoa(request.Port)),
		NetID:      request.NetID,
		AMSPort:    request.AMSPort,
		Symbol:     request.Symbol,
		Value:      value,
		DurationMS: time.Since(started).Milliseconds(),
	}
	if symbol != nil {
		result.DataType = symbol.DataType
		result.Length = symbol.Length
	}
	return result, nil
}

func TestBACnet(ctx context.Context, request BACnetProbeRequest) (ProbeResult, error) {
	request.Host = strings.TrimSpace(request.Host)
	if request.Host == "" {
		return ProbeResult{}, fmt.Errorf("host is required")
	}
	if request.Port == 0 {
		request.Port = defaultBACnetPort
	}
	if request.Port < 1 || request.Port > 65535 {
		return ProbeResult{}, fmt.Errorf("port must be between 1 and 65535")
	}

	started := time.Now()
	conn, err := net.DialTimeout("udp", net.JoinHostPort(request.Host, strconv.Itoa(request.Port)), timeoutDuration(request.TimeoutMS))
	if err != nil {
		return ProbeResult{}, fmt.Errorf("open bacnet udp endpoint: %w", err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(timeoutDuration(request.TimeoutMS)))
	}

	if _, err := conn.Write(bacnetWhoIsFrame()); err != nil {
		return ProbeResult{}, fmt.Errorf("send bacnet who-is: %w", err)
	}
	if request.ExpectResponse {
		buffer := make([]byte, 512)
		if _, err := conn.Read(buffer); err != nil {
			return ProbeResult{}, fmt.Errorf("wait bacnet response: %w", err)
		}
	}

	status := "probe_sent"
	message := "BACnet/IP Who-Is probe sent over UDP. Use /api/v1/protocols/bacnet/read-property for property reads."
	if request.ExpectResponse {
		status = "response_received"
		message = "BACnet/IP endpoint responded to the UDP probe."
	}
	return ProbeResult{
		OK:         true,
		Protocol:   "bacnet",
		Status:     status,
		Endpoint:   net.JoinHostPort(request.Host, strconv.Itoa(request.Port)),
		Message:    message,
		DurationMS: time.Since(started).Milliseconds(),
	}, nil
}

func ReadBACnetProperty(ctx context.Context, request BACnetReadPropertyRequest) (BACnetReadPropertyResult, error) {
	started := time.Now()
	request.Host = strings.TrimSpace(request.Host)
	if request.Host == "" {
		return BACnetReadPropertyResult{}, fmt.Errorf("host is required")
	}
	if request.Port == 0 {
		request.Port = defaultBACnetPort
	}
	if request.PropertyID == 0 {
		propertyID, err := bacnetPropertyID(request.Property)
		if err != nil {
			return BACnetReadPropertyResult{}, err
		}
		request.PropertyID = propertyID
	}
	if request.ObjectTypeID == 0 && strings.TrimSpace(request.ObjectType) != "" {
		objectTypeID, err := bacnetObjectTypeID(request.ObjectType)
		if err != nil {
			return BACnetReadPropertyResult{}, err
		}
		request.ObjectTypeID = objectTypeID
	}

	conn, err := net.DialTimeout("udp", net.JoinHostPort(request.Host, strconv.Itoa(request.Port)), timeoutDuration(request.TimeoutMS))
	if err != nil {
		return BACnetReadPropertyResult{}, fmt.Errorf("open bacnet udp endpoint: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(timeoutDuration(request.TimeoutMS)))
	}

	invokeID := byte(time.Now().UnixNano())
	frame, err := bacnetReadPropertyFrame(invokeID, request.ObjectTypeID, request.ObjectInstance, request.PropertyID, request.ArrayIndex)
	if err != nil {
		return BACnetReadPropertyResult{}, err
	}
	if _, err := conn.Write(frame); err != nil {
		return BACnetReadPropertyResult{}, fmt.Errorf("send bacnet readproperty: %w", err)
	}
	buffer := make([]byte, 1476)
	n, err := conn.Read(buffer)
	if err != nil {
		return BACnetReadPropertyResult{}, fmt.Errorf("read bacnet response: %w", err)
	}
	value, valueType, raw, err := parseBACnetReadPropertyResponse(buffer[:n], invokeID)
	if err != nil {
		return BACnetReadPropertyResult{}, err
	}
	return BACnetReadPropertyResult{
		OK:             true,
		Protocol:       "bacnet",
		Endpoint:       net.JoinHostPort(request.Host, strconv.Itoa(request.Port)),
		ObjectTypeID:   request.ObjectTypeID,
		ObjectInstance: request.ObjectInstance,
		PropertyID:     request.PropertyID,
		Value:          value,
		ValueType:      valueType,
		RawHex:         hex.EncodeToString(raw),
		DurationMS:     time.Since(started).Milliseconds(),
	}, nil
}

func TestModbusRTU(request ModbusRTUProbeRequest) (ProbeResult, error) {
	started := time.Now()
	mode, err := modbusRTUMode(request.BaudRate, request.DataBits, request.Parity, request.StopBits)
	if err != nil {
		return ProbeResult{}, err
	}
	portName := strings.TrimSpace(request.Port)
	if portName == "" {
		return ProbeResult{}, fmt.Errorf("serial port is required")
	}
	port, err := serial.Open(portName, mode)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("open serial port: %w", err)
	}
	_ = port.Close()

	return ProbeResult{
		OK:         true,
		Protocol:   "modbus_rtu",
		Status:     "port_opened",
		Endpoint:   portName,
		Message:    fmt.Sprintf("Serial port opened with %d-%d-%s-%d settings.", mode.BaudRate, mode.DataBits, strings.ToLower(request.Parity), request.StopBits),
		DurationMS: time.Since(started).Milliseconds(),
	}, nil
}

func ReadModbusRTURegisters(ctx context.Context, request ModbusRTUReadRequest) (ModbusRTUReadResult, error) {
	normalized, err := normalizeModbusRTUReadRequest(request)
	if err != nil {
		return ModbusRTUReadResult{}, err
	}
	mode, err := modbusRTUMode(normalized.BaudRate, normalized.DataBits, normalized.Parity, normalized.StopBits)
	if err != nil {
		return ModbusRTUReadResult{}, err
	}
	started := time.Now()
	port, err := serial.Open(normalized.Port, mode)
	if err != nil {
		return ModbusRTUReadResult{}, fmt.Errorf("open serial port: %w", err)
	}
	defer port.Close()
	if err := port.SetReadTimeout(timeoutDuration(normalized.TimeoutMS)); err != nil {
		return ModbusRTUReadResult{}, fmt.Errorf("set serial timeout: %w", err)
	}
	_ = port.ResetInputBuffer()
	_ = port.ResetOutputBuffer()

	frame := modbusRTUReadFrame(normalized.UnitID, normalized.FunctionCode, normalized.Address, normalized.Quantity)
	if _, err := port.Write(frame); err != nil {
		return ModbusRTUReadResult{}, fmt.Errorf("write modbus rtu frame: %w", err)
	}

	responseLength := 5 + int(normalized.Quantity)*2
	response := make([]byte, responseLength)
	if err := readFullWithContext(ctx, port, response); err != nil {
		return ModbusRTUReadResult{}, fmt.Errorf("read modbus rtu response: %w", err)
	}
	registers, data, err := parseModbusRTUReadResponse(response, normalized.UnitID, normalized.FunctionCode, normalized.Quantity)
	if err != nil {
		return ModbusRTUReadResult{}, err
	}
	return ModbusRTUReadResult{
		OK:           true,
		Protocol:     "modbus_rtu",
		Endpoint:     normalized.Port,
		UnitID:       normalized.UnitID,
		FunctionCode: normalized.FunctionCode,
		Address:      normalized.Address,
		Quantity:     normalized.Quantity,
		Registers:    registers,
		Bytes:        byteInts(data),
		DurationMS:   time.Since(started).Milliseconds(),
	}, nil
}

func TestODBC(ctx context.Context, request ODBCProbeRequest) (ProbeResult, error) {
	started := time.Now()
	connectionString, endpoint, err := buildODBCConnectionString(request.DSN, request.Driver, request.ConnectionString)
	if err != nil {
		return ProbeResult{}, err
	}
	db, err := sql.Open("odbc", connectionString)
	if err != nil {
		return ProbeResult{}, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	pingCtx, cancel := context.WithTimeout(ctx, timeoutDuration(request.TimeoutMS))
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return ProbeResult{}, fmt.Errorf("odbc ping: %w", err)
	}
	return ProbeResult{
		OK:         true,
		Protocol:   "odbc",
		Status:     "connected",
		Endpoint:   endpoint,
		Message:    "ODBC data source is reachable.",
		DurationMS: time.Since(started).Milliseconds(),
	}, nil
}

func QueryODBC(ctx context.Context, request ODBCQueryRequest) (ODBCQueryResult, error) {
	started := time.Now()
	connectionString, endpoint, err := buildODBCConnectionString(request.DSN, request.Driver, request.ConnectionString)
	if err != nil {
		return ODBCQueryResult{}, err
	}
	query := strings.TrimSpace(request.Query)
	if err := validateReadOnlyQuery(query); err != nil {
		return ODBCQueryResult{}, err
	}
	maxRows := request.MaxRows
	if maxRows <= 0 {
		maxRows = defaultODBCMaxRows
	}
	if maxRows > maxODBCRows {
		maxRows = maxODBCRows
	}

	db, err := sql.Open("odbc", connectionString)
	if err != nil {
		return ODBCQueryResult{}, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	queryCtx, cancel := context.WithTimeout(ctx, timeoutDuration(request.TimeoutMS))
	defer cancel()

	rows, err := db.QueryContext(queryCtx, query)
	if err != nil {
		return ODBCQueryResult{}, fmt.Errorf("odbc query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return ODBCQueryResult{}, fmt.Errorf("read odbc columns: %w", err)
	}
	resultRows := make([]map[string]any, 0)
	for rows.Next() {
		if len(resultRows) >= maxRows {
			break
		}
		values := make([]any, len(columns))
		scanTargets := make([]any, len(columns))
		for i := range values {
			scanTargets[i] = &values[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return ODBCQueryResult{}, fmt.Errorf("scan odbc row: %w", err)
		}
		row := make(map[string]any, len(columns))
		for i, column := range columns {
			row[column] = normalizeSQLValue(values[i])
		}
		resultRows = append(resultRows, row)
	}
	if err := rows.Err(); err != nil {
		return ODBCQueryResult{}, fmt.Errorf("iterate odbc rows: %w", err)
	}
	return ODBCQueryResult{
		OK:         true,
		Protocol:   "odbc",
		Endpoint:   endpoint,
		Columns:    columns,
		Rows:       resultRows,
		RowCount:   len(resultRows),
		DurationMS: time.Since(started).Milliseconds(),
	}, nil
}

func testTCP(ctx context.Context, protocol string, request TCPProbeRequest) (ProbeResult, error) {
	request.Host = strings.TrimSpace(request.Host)
	if request.Host == "" {
		return ProbeResult{}, fmt.Errorf("host is required")
	}
	if request.Port == 0 {
		return ProbeResult{}, fmt.Errorf("port is required")
	}
	if request.Port < 1 || request.Port > 65535 {
		return ProbeResult{}, fmt.Errorf("port must be between 1 and 65535")
	}
	started := time.Now()
	dialer := net.Dialer{Timeout: timeoutDuration(request.TimeoutMS)}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(request.Host, strconv.Itoa(request.Port)))
	if err != nil {
		return ProbeResult{}, fmt.Errorf("connect %s tcp: %w", protocol, err)
	}
	_ = conn.Close()
	return ProbeResult{
		OK:         true,
		Protocol:   protocol,
		Status:     "tcp_reachable",
		Endpoint:   net.JoinHostPort(request.Host, strconv.Itoa(request.Port)),
		DurationMS: time.Since(started).Milliseconds(),
	}, nil
}

func (request OPCUAProbeRequest) opcuaTCPRequest() (TCPProbeRequest, error) {
	return opcuaTCPRequest(request.EndpointURL, request.Host, request.Port, request.TimeoutMS)
}

func opcuaTCPRequest(endpointURL string, host string, port int, timeoutMS int) (TCPProbeRequest, error) {
	if strings.TrimSpace(endpointURL) == "" {
		if port == 0 {
			port = defaultOPCUAPort
		}
		return TCPProbeRequest{Host: host, Port: port, TimeoutMS: timeoutMS}, nil
	}
	parsed, err := url.Parse(endpointURL)
	if err != nil {
		return TCPProbeRequest{}, fmt.Errorf("parse endpoint_url: %w", err)
	}
	host = parsed.Hostname()
	port = defaultOPCUAPort
	if parsed.Port() != "" {
		parsedPort, err := strconv.Atoi(parsed.Port())
		if err != nil {
			return TCPProbeRequest{}, fmt.Errorf("parse endpoint port: %w", err)
		}
		port = parsedPort
	}
	return TCPProbeRequest{Host: host, Port: port, TimeoutMS: timeoutMS}, nil
}

func connectOPCUA(ctx context.Context, endpointURL string, host string, port int, timeoutMS int, username string, password string) (*opcua.Client, string, error) {
	if strings.TrimSpace(endpointURL) == "" {
		tcpRequest, err := opcuaTCPRequest("", host, port, timeoutMS)
		if err != nil {
			return nil, "", err
		}
		if strings.TrimSpace(tcpRequest.Host) == "" {
			return nil, "", fmt.Errorf("endpoint_url or host is required")
		}
		endpointURL = "opc.tcp://" + net.JoinHostPort(tcpRequest.Host, strconv.Itoa(tcpRequest.Port))
	}
	timeout := timeoutDuration(timeoutMS)
	options := []opcua.Option{
		opcua.SecurityMode(ua.MessageSecurityModeNone),
		opcua.SecurityPolicy(ua.SecurityPolicyURINone),
		opcua.RequestTimeout(timeout),
		opcua.DialTimeout(timeout),
		opcua.AutoReconnect(false),
	}
	if strings.TrimSpace(username) != "" {
		options = append(options, opcua.AuthUsername(username, password))
	} else {
		options = append(options, opcua.AuthAnonymous())
	}
	client, err := opcua.NewClient(endpointURL, options...)
	if err != nil {
		return nil, "", fmt.Errorf("create opcua client: %w", err)
	}
	if err := client.Connect(ctx); err != nil {
		return nil, "", fmt.Errorf("connect opcua: %w", err)
	}
	return client, endpointURL, nil
}

func opcuaAttributeID(raw string, explicit uint32) (ua.AttributeID, string, error) {
	if explicit != 0 {
		return ua.AttributeID(explicit), ua.AttributeID(explicit).String(), nil
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "value":
		return ua.AttributeIDValue, "value", nil
	case "node_id", "nodeid":
		return ua.AttributeIDNodeID, "node_id", nil
	case "node_class", "nodeclass":
		return ua.AttributeIDNodeClass, "node_class", nil
	case "browse_name", "browsename":
		return ua.AttributeIDBrowseName, "browse_name", nil
	case "display_name", "displayname":
		return ua.AttributeIDDisplayName, "display_name", nil
	case "description":
		return ua.AttributeIDDescription, "description", nil
	case "data_type", "datatype":
		return ua.AttributeIDDataType, "data_type", nil
	case "access_level", "accesslevel":
		return ua.AttributeIDAccessLevel, "access_level", nil
	default:
		return 0, "", fmt.Errorf("unsupported opcua attribute: %s", raw)
	}
}

func opcuaValue(value *ua.Variant) (any, string) {
	if value == nil {
		return nil, ""
	}
	raw := value.Value()
	if raw == nil {
		return nil, value.Type().String()
	}
	return raw, fmt.Sprintf("%T", raw)
}

func localizedText(value *ua.LocalizedText) string {
	if value == nil {
		return ""
	}
	return value.Text
}

func qualifiedName(value *ua.QualifiedName) string {
	if value == nil {
		return ""
	}
	if value.NamespaceIndex == 0 {
		return value.Name
	}
	return fmt.Sprintf("%d:%s", value.NamespaceIndex, value.Name)
}

func stringer(value fmt.Stringer) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func timeOrEmpty(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func timeoutDuration(timeoutMS int) time.Duration {
	if timeoutMS <= 0 {
		timeoutMS = defaultTimeoutMS
	}
	return time.Duration(timeoutMS) * time.Millisecond
}

func bacnetWhoIsFrame() []byte {
	return []byte{
		0x81, 0x0a, 0x00, 0x0c,
		0x01, 0x20,
		0x10, 0x08,
		0x00, 0x00, 0x3f, 0xff,
	}
}

func bacnetReadPropertyFrame(invokeID byte, objectTypeID uint16, objectInstance uint32, propertyID uint32, arrayIndex *uint32) ([]byte, error) {
	if objectTypeID > 1023 {
		return nil, fmt.Errorf("object_type_id must be between 0 and 1023")
	}
	if objectInstance > 0x3fffff {
		return nil, fmt.Errorf("object_instance must be between 0 and 4194303")
	}
	apdu := []byte{0x00, 0x05, invokeID, 0x0c}
	apdu = append(apdu, bacnetContextObjectID(0, objectTypeID, objectInstance)...)
	apdu = append(apdu, bacnetContextUnsigned(1, propertyID)...)
	if arrayIndex != nil {
		apdu = append(apdu, bacnetContextUnsigned(2, *arrayIndex)...)
	}
	frame := []byte{0x81, 0x0a, 0x00, 0x00, 0x01, 0x04}
	frame = append(frame, apdu...)
	binary.BigEndian.PutUint16(frame[2:4], uint16(len(frame)))
	return frame, nil
}

func bacnetContextObjectID(tag byte, objectTypeID uint16, objectInstance uint32) []byte {
	encoded := (uint32(objectTypeID) << 22) | objectInstance
	return []byte{
		(tag << 4) | 0x08 | 4,
		byte(encoded >> 24),
		byte(encoded >> 16),
		byte(encoded >> 8),
		byte(encoded),
	}
}

func bacnetContextUnsigned(tag byte, value uint32) []byte {
	data := unsignedBytes(value)
	header := (tag << 4) | 0x08 | byte(len(data))
	return append([]byte{header}, data...)
}

func unsignedBytes(value uint32) []byte {
	switch {
	case value <= 0xff:
		return []byte{byte(value)}
	case value <= 0xffff:
		return []byte{byte(value >> 8), byte(value)}
	case value <= 0xffffff:
		return []byte{byte(value >> 16), byte(value >> 8), byte(value)}
	default:
		return []byte{byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value)}
	}
}

func parseBACnetReadPropertyResponse(frame []byte, invokeID byte) (any, string, []byte, error) {
	if len(frame) < 9 {
		return nil, "", nil, fmt.Errorf("short bacnet response")
	}
	if frame[0] != 0x81 {
		return nil, "", nil, fmt.Errorf("not a bacnet/ip bvlc response")
	}
	apduOffset := 6
	apdu := frame[apduOffset:]
	if len(apdu) < 3 {
		return nil, "", nil, fmt.Errorf("short bacnet apdu")
	}
	pduType := apdu[0] & 0xf0
	if pduType == 0x50 {
		return nil, "", nil, fmt.Errorf("bacnet error response: %s", hex.EncodeToString(apdu))
	}
	if pduType != 0x30 {
		return nil, "", nil, fmt.Errorf("expected bacnet complex ack, got pdu 0x%02x", apdu[0])
	}
	if apdu[1] != invokeID {
		return nil, "", nil, fmt.Errorf("unexpected bacnet invoke id: got %d want %d", apdu[1], invokeID)
	}
	if apdu[2] != 0x0c {
		return nil, "", nil, fmt.Errorf("unexpected bacnet service choice: %d", apdu[2])
	}
	payload := apdu[3:]
	start := -1
	end := -1
	for i, b := range payload {
		if b == 0x3e && start == -1 {
			start = i + 1
			continue
		}
		if b == 0x3f && start != -1 {
			end = i
			break
		}
	}
	if start == -1 || end == -1 || end <= start {
		return nil, "", payload, fmt.Errorf("bacnet property value tag not found")
	}
	raw := payload[start:end]
	value, valueType, err := parseBACnetApplicationValue(raw)
	if err != nil {
		return nil, "", raw, err
	}
	return value, valueType, raw, nil
}

func parseBACnetApplicationValue(data []byte) (any, string, error) {
	if len(data) == 0 {
		return nil, "", fmt.Errorf("empty bacnet application value")
	}
	tag := data[0] >> 4
	lvt := data[0] & 0x07
	if tag == 1 {
		return lvt == 1, "boolean", nil
	}
	length := int(lvt)
	offset := 1
	if lvt == 5 {
		if len(data) < 2 {
			return nil, "", fmt.Errorf("short bacnet extended length")
		}
		length = int(data[1])
		offset = 2
	}
	if len(data) < offset+length {
		return nil, "", fmt.Errorf("short bacnet application value")
	}
	valueData := data[offset : offset+length]
	switch tag {
	case 2:
		return decodeUnsigned(valueData), "unsigned", nil
	case 3:
		return decodeSigned(valueData), "signed", nil
	case 4:
		if len(valueData) != 4 {
			return nil, "", fmt.Errorf("invalid bacnet real length")
		}
		return math.Float32frombits(binary.BigEndian.Uint32(valueData)), "real", nil
	case 5:
		if len(valueData) != 8 {
			return nil, "", fmt.Errorf("invalid bacnet double length")
		}
		return math.Float64frombits(binary.BigEndian.Uint64(valueData)), "double", nil
	case 7:
		if len(valueData) == 0 {
			return "", "character_string", nil
		}
		return string(valueData[1:]), "character_string", nil
	case 9:
		return decodeUnsigned(valueData), "enumerated", nil
	case 12:
		if len(valueData) != 4 {
			return nil, "", fmt.Errorf("invalid bacnet object id length")
		}
		raw := binary.BigEndian.Uint32(valueData)
		return map[string]any{
			"object_type_id": raw >> 22,
			"instance":       raw & 0x3fffff,
		}, "object_identifier", nil
	default:
		return hex.EncodeToString(valueData), fmt.Sprintf("application_tag_%d", tag), nil
	}
}

func decodeUnsigned(data []byte) uint64 {
	var value uint64
	for _, b := range data {
		value = (value << 8) | uint64(b)
	}
	return value
}

func decodeSigned(data []byte) int64 {
	if len(data) == 0 {
		return 0
	}
	unsigned := decodeUnsigned(data)
	shift := uint(64 - len(data)*8)
	return int64(unsigned<<shift) >> shift
}

func bacnetObjectTypeID(value string) (uint16, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "analog_input", "analog-input", "ai", "0":
		return 0, nil
	case "analog_output", "analog-output", "ao", "1":
		return 1, nil
	case "analog_value", "analog-value", "av", "2":
		return 2, nil
	case "binary_input", "binary-input", "bi", "3":
		return 3, nil
	case "binary_output", "binary-output", "bo", "4":
		return 4, nil
	case "binary_value", "binary-value", "bv", "5":
		return 5, nil
	case "device", "8":
		return 8, nil
	case "multi_state_input", "multi-state-input", "msi", "13":
		return 13, nil
	case "multi_state_output", "multi-state-output", "mso", "14":
		return 14, nil
	case "multi_state_value", "multi-state-value", "msv", "19":
		return 19, nil
	default:
		id, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return 0, fmt.Errorf("unsupported bacnet object_type: %s", value)
		}
		return uint16(id), nil
	}
}

func bacnetPropertyID(value string) (uint32, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "present_value", "present-value", "pv", "85":
		return 85, nil
	case "object_identifier", "object-identifier", "75":
		return 75, nil
	case "object_name", "object-name", "77":
		return 77, nil
	case "description", "28":
		return 28, nil
	case "status_flags", "status-flags", "111":
		return 111, nil
	case "units", "117":
		return 117, nil
	default:
		id, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("unsupported bacnet property: %s", value)
		}
		return uint32(id), nil
	}
}

func modbusRTUMode(baudRate int, dataBits int, parity string, stopBits int) (*serial.Mode, error) {
	if baudRate == 0 {
		baudRate = 9600
	}
	if dataBits == 0 {
		dataBits = 8
	}
	if stopBits == 0 {
		stopBits = 1
	}
	mode := &serial.Mode{
		BaudRate: baudRate,
		DataBits: dataBits,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	switch strings.ToLower(strings.TrimSpace(parity)) {
	case "", "none", "n":
		mode.Parity = serial.NoParity
	case "odd", "o":
		mode.Parity = serial.OddParity
	case "even", "e":
		mode.Parity = serial.EvenParity
	case "mark", "m":
		mode.Parity = serial.MarkParity
	case "space", "s":
		mode.Parity = serial.SpaceParity
	default:
		return nil, fmt.Errorf("unsupported parity: %s", parity)
	}
	switch stopBits {
	case 1:
		mode.StopBits = serial.OneStopBit
	case 2:
		mode.StopBits = serial.TwoStopBits
	default:
		return nil, fmt.Errorf("unsupported stop_bits: %d", stopBits)
	}
	return mode, nil
}

func normalizeModbusRTUReadRequest(request ModbusRTUReadRequest) (ModbusRTUReadRequest, error) {
	request.Port = strings.TrimSpace(request.Port)
	if request.Port == "" {
		return ModbusRTUReadRequest{}, fmt.Errorf("serial port is required")
	}
	if request.UnitID == 0 {
		request.UnitID = 1
	}
	if request.Quantity == 0 || request.Quantity > maxModbusRTURegisters {
		return ModbusRTUReadRequest{}, fmt.Errorf("quantity must be between 1 and %d", maxModbusRTURegisters)
	}
	if request.BaudRate == 0 {
		request.BaudRate = 9600
	}
	if request.DataBits == 0 {
		request.DataBits = 8
	}
	if request.StopBits == 0 {
		request.StopBits = 1
	}
	if request.Parity == "" {
		request.Parity = "none"
	}
	functionCode := request.FunctionCode
	if functionCode == 0 {
		parsed, err := parseModbusFunctionCode(request.Function)
		if err != nil {
			return ModbusRTUReadRequest{}, err
		}
		functionCode = parsed
	}
	if functionCode != 3 && functionCode != 4 {
		return ModbusRTUReadRequest{}, fmt.Errorf("only function codes 3 and 4 are supported")
	}
	request.FunctionCode = functionCode
	if request.TimeoutMS <= 0 {
		request.TimeoutMS = defaultTimeoutMS
	}
	return request, nil
}

func parseModbusFunctionCode(value string) (uint8, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "holding", "holding_register", "holding_registers", "read_holding_registers", "3", "03":
		return 3, nil
	case "input", "input_register", "input_registers", "read_input_registers", "4", "04":
		return 4, nil
	default:
		return 0, fmt.Errorf("unsupported function: %s", value)
	}
}

func modbusRTUReadFrame(unitID uint8, functionCode uint8, address uint16, quantity uint16) []byte {
	frame := make([]byte, 8)
	frame[0] = unitID
	frame[1] = functionCode
	binary.BigEndian.PutUint16(frame[2:4], address)
	binary.BigEndian.PutUint16(frame[4:6], quantity)
	crc := modbusCRC(frame[:6])
	binary.LittleEndian.PutUint16(frame[6:8], crc)
	return frame
}

func parseModbusRTUReadResponse(response []byte, unitID uint8, functionCode uint8, quantity uint16) ([]uint16, []byte, error) {
	if len(response) < 5 {
		return nil, nil, fmt.Errorf("short modbus rtu response")
	}
	if !validModbusCRC(response) {
		return nil, nil, fmt.Errorf("invalid modbus rtu crc")
	}
	if response[0] != unitID {
		return nil, nil, fmt.Errorf("unexpected unit id: got %d want %d", response[0], unitID)
	}
	if response[1]&0x80 != 0 {
		return nil, nil, fmt.Errorf("modbus rtu exception code %d", response[2])
	}
	if response[1] != functionCode {
		return nil, nil, fmt.Errorf("unexpected function code: got %d want %d", response[1], functionCode)
	}
	byteCount := int(response[2])
	expectedByteCount := int(quantity) * 2
	if byteCount != expectedByteCount {
		return nil, nil, fmt.Errorf("unexpected byte count: got %d want %d", byteCount, expectedByteCount)
	}
	if len(response) != 3+byteCount+2 {
		return nil, nil, fmt.Errorf("unexpected response length")
	}
	data := response[3 : 3+byteCount]
	registers := make([]uint16, 0, quantity)
	for index := 0; index < byteCount; index += 2 {
		registers = append(registers, binary.BigEndian.Uint16(data[index:index+2]))
	}
	return registers, data, nil
}

func modbusCRC(data []byte) uint16 {
	crc := uint16(0xffff)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xa001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func validModbusCRC(frame []byte) bool {
	if len(frame) < 3 {
		return false
	}
	got := binary.LittleEndian.Uint16(frame[len(frame)-2:])
	return got == modbusCRC(frame[:len(frame)-2])
}

type serialReader interface {
	Read([]byte) (int, error)
}

func readFullWithContext(ctx context.Context, reader serialReader, target []byte) error {
	done := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(reader, target)
		done <- err
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func netIDFromHost(host string) string {
	ip := net.ParseIP(host).To4()
	if ip == nil {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d.%d.1.1", ip[0], ip[1], ip[2], ip[3])
}

func buildODBCConnectionString(dsn string, driver string, connectionString string) (string, string, error) {
	if strings.TrimSpace(connectionString) != "" {
		return strings.TrimSpace(connectionString), "connection_string", nil
	}
	dsn = strings.TrimSpace(dsn)
	driver = strings.TrimSpace(driver)
	if dsn == "" && driver == "" {
		return "", "", fmt.Errorf("dsn, driver, or connection_string is required")
	}
	parts := make([]string, 0, 2)
	if driver != "" {
		parts = append(parts, fmt.Sprintf("Driver={%s}", strings.Trim(driver, "{}")))
	}
	if dsn != "" {
		parts = append(parts, "DSN="+dsn)
	}
	return strings.Join(parts, ";") + ";", firstNonEmpty(dsn, driver), nil
}

func validateReadOnlyQuery(query string) error {
	if query == "" {
		return fmt.Errorf("query is required")
	}
	normalized := strings.ToLower(strings.TrimSpace(query))
	allowedPrefixes := []string{"select", "with", "show", "describe", "explain"}
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(normalized, prefix) {
			return nil
		}
	}
	return fmt.Errorf("only read-only ODBC queries are allowed")
}

func normalizeSQLValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(v)
	case time.Time:
		return v.Format(time.RFC3339Nano)
	default:
		return v
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func byteInts(data []byte) []int {
	result := make([]int, 0, len(data))
	for _, value := range data {
		result = append(result, int(value))
	}
	return result
}
