from __future__ import annotations

import json

from PySide6.QtWidgets import (
    QComboBox,
    QFrame,
    QHBoxLayout,
    QLabel,
    QLineEdit,
    QMessageBox,
    QPlainTextEdit,
    QPushButton,
    QSpinBox,
    QTableWidget,
    QTableWidgetItem,
    QVBoxLayout,
    QWidget,
)


class ProtocolSettingsPage(QWidget):
    def __init__(self, api_client) -> None:
        super().__init__()
        self.api_client = api_client

        layout = QVBoxLayout(self)
        layout.addWidget(self._build_status_panel(), 1)
        layout.addWidget(self._build_mqtt_test_panel(), 1)
        layout.addWidget(self._build_modbus_test_panel(), 1)
        layout.addWidget(self._build_industrial_test_panel(), 1)

    def _build_status_panel(self) -> QWidget:
        panel = QFrame()
        panel.setProperty("card", True)
        layout = QVBoxLayout(panel)

        title = QLabel("Protocol Connectors")
        title.setProperty("accent", True)
        hint = QLabel("MQTT status, bridge readiness, and simulator state are checked from the local Go backend.")
        hint.setProperty("muted", True)

        self.table = QTableWidget(0, 5)
        self.table.setHorizontalHeaderLabels(["Protocol", "Enabled", "Status", "Extra", "Last error"])

        layout.addWidget(title)
        layout.addWidget(hint)
        layout.addWidget(self.table, 1)
        return panel

    def _build_mqtt_test_panel(self) -> QWidget:
        panel = QFrame()
        panel.setProperty("glass", True)
        layout = QVBoxLayout(panel)

        title = QLabel("MQTT Test Console")
        title.setProperty("accent", True)
        hint = QLabel("Test broker connectivity, then publish a real telemetry payload into the configured MQTT flow.")
        hint.setProperty("muted", True)

        row = QHBoxLayout()
        self.topic_input = QLineEdit("iot/demo/mqtt_test_001/telemetry")
        test_button = QPushButton("Test Connection")
        test_button.setProperty("accent", True)
        test_button.clicked.connect(self._test_connection)
        publish_button = QPushButton("Publish Test Payload")
        publish_button.clicked.connect(self._publish_test)
        row.addWidget(QLabel("Topic"))
        row.addWidget(self.topic_input, 1)
        row.addWidget(test_button)
        row.addWidget(publish_button)

        self.payload_editor = QPlainTextEdit()
        self.payload_editor.setPlainText(
            json.dumps(
                {
                    "device_id": "mqtt_test_001",
                    "name": "MQTT Test Temperature",
                    "metric": "temperature",
                    "value": 24.8,
                    "unit": "C",
                },
                indent=2,
            )
        )
        self.result_label = QLabel("Ready to test MQTT.")
        self.result_label.setProperty("muted", True)

        layout.addWidget(title)
        layout.addWidget(hint)
        layout.addLayout(row)
        layout.addWidget(self.payload_editor, 1)
        layout.addWidget(self.result_label)
        return panel

    def _build_modbus_test_panel(self) -> QWidget:
        panel = QFrame()
        panel.setProperty("glass", True)
        layout = QVBoxLayout(panel)

        title = QLabel("Modbus Web API Console")
        title.setProperty("accent", True)
        hint = QLabel("Test a Modbus TCP device and read raw holding/input registers through the local Go backend.")
        hint.setProperty("muted", True)

        connection_row = QHBoxLayout()
        self.modbus_host_input = QLineEdit("127.0.0.1")
        self.modbus_port_input = QSpinBox()
        self.modbus_port_input.setRange(1, 65535)
        self.modbus_port_input.setValue(502)
        self.modbus_unit_input = QSpinBox()
        self.modbus_unit_input.setRange(1, 247)
        self.modbus_unit_input.setValue(1)
        self.modbus_timeout_input = QSpinBox()
        self.modbus_timeout_input.setRange(250, 30000)
        self.modbus_timeout_input.setValue(3000)
        connection_row.addWidget(QLabel("Host"))
        connection_row.addWidget(self.modbus_host_input, 1)
        connection_row.addWidget(QLabel("Port"))
        connection_row.addWidget(self.modbus_port_input)
        connection_row.addWidget(QLabel("Unit"))
        connection_row.addWidget(self.modbus_unit_input)
        connection_row.addWidget(QLabel("Timeout ms"))
        connection_row.addWidget(self.modbus_timeout_input)

        read_row = QHBoxLayout()
        self.modbus_function_combo = QComboBox()
        self.modbus_function_combo.addItems(["holding_registers", "input_registers"])
        self.modbus_address_input = QSpinBox()
        self.modbus_address_input.setRange(0, 65535)
        self.modbus_address_input.setValue(0)
        self.modbus_quantity_input = QSpinBox()
        self.modbus_quantity_input.setRange(1, 125)
        self.modbus_quantity_input.setValue(2)
        test_button = QPushButton("Test TCP")
        test_button.setProperty("accent", True)
        test_button.clicked.connect(self._test_modbus_connection)
        read_button = QPushButton("Read Registers")
        read_button.clicked.connect(self._read_modbus_registers)
        read_row.addWidget(QLabel("Function"))
        read_row.addWidget(self.modbus_function_combo)
        read_row.addWidget(QLabel("Address"))
        read_row.addWidget(self.modbus_address_input)
        read_row.addWidget(QLabel("Quantity"))
        read_row.addWidget(self.modbus_quantity_input)
        read_row.addWidget(test_button)
        read_row.addWidget(read_button)

        self.modbus_result = QPlainTextEdit()
        self.modbus_result.setReadOnly(True)
        self.modbus_result.setPlainText("Ready to call Modbus Web API.")

        layout.addWidget(title)
        layout.addWidget(hint)
        layout.addLayout(connection_row)
        layout.addLayout(read_row)
        layout.addWidget(self.modbus_result, 1)
        return panel

    def _build_industrial_test_panel(self) -> QWidget:
        panel = QFrame()
        panel.setProperty("glass", True)
        layout = QVBoxLayout(panel)

        title = QLabel("Industrial Protocol Console")
        title.setProperty("accent", True)
        hint = QLabel("OPC UA browse/read, BACnet ReadProperty, Modbus RTU read, ADS symbols, and ODBC live query.")
        hint.setProperty("muted", True)

        row = QHBoxLayout()
        self.industrial_protocol_combo = QComboBox()
        self.industrial_protocol_combo.addItems(["opcua", "bacnet", "modbus_rtu", "ads", "odbc"])
        self.industrial_protocol_combo.currentTextChanged.connect(self._refresh_industrial_actions)
        self.industrial_action_combo = QComboBox()
        self.industrial_action_combo.currentTextChanged.connect(self._load_industrial_payload_template)
        run_button = QPushButton("Run Action")
        run_button.setProperty("accent", True)
        run_button.clicked.connect(self._run_industrial_action)
        row.addWidget(QLabel("Protocol"))
        row.addWidget(self.industrial_protocol_combo)
        row.addWidget(QLabel("Action"))
        row.addWidget(self.industrial_action_combo)
        row.addWidget(run_button)
        row.addStretch(1)

        self.industrial_payload_editor = QPlainTextEdit()
        self.industrial_result = QPlainTextEdit()
        self.industrial_result.setReadOnly(True)
        self.industrial_result.setPlainText("Choose a protocol/action and run it through the backend.")
        self._refresh_industrial_actions()

        layout.addWidget(title)
        layout.addWidget(hint)
        layout.addLayout(row)
        layout.addWidget(QLabel("Request JSON"))
        layout.addWidget(self.industrial_payload_editor, 1)
        layout.addWidget(QLabel("Result"))
        layout.addWidget(self.industrial_result, 1)
        return panel

    def refresh(self) -> None:
        try:
            status = self.api_client.protocol_status()
        except RuntimeError as exc:
            self.table.setRowCount(1)
            self.table.setItem(0, 0, QTableWidgetItem("error"))
            self.table.setItem(0, 1, QTableWidgetItem("-"))
            self.table.setItem(0, 2, QTableWidgetItem(str(exc)))
            self.table.setItem(0, 3, QTableWidgetItem("-"))
            self.table.setItem(0, 4, QTableWidgetItem("-"))
            return

        self.table.setRowCount(len(status))
        for row, (protocol, values) in enumerate(status.items()):
            extra = values.get("subscription") or values.get("broker") or values.get("timeout") or "-"
            self.table.setItem(row, 0, QTableWidgetItem(protocol))
            self.table.setItem(row, 1, QTableWidgetItem(str(values.get("enabled", False))))
            self.table.setItem(row, 2, QTableWidgetItem(str(values.get("status", "unknown"))))
            self.table.setItem(row, 3, QTableWidgetItem(str(extra)))
            self.table.setItem(row, 4, QTableWidgetItem(str(values.get("last_error", ""))))

    def _test_connection(self) -> None:
        try:
            response = self.api_client.mqtt_test_connection()
            self.result_label.setText(f"MQTT broker reachable: {response.get('broker')} ({response.get('status')})")
            self.refresh()
        except RuntimeError as exc:
            self.result_label.setText(f"MQTT connection failed: {exc}")

    def _publish_test(self) -> None:
        try:
            payload = json.loads(self.payload_editor.toPlainText())
        except json.JSONDecodeError as exc:
            QMessageBox.warning(self, "Invalid JSON", str(exc))
            return
        try:
            response = self.api_client.mqtt_publish_test(self.topic_input.text(), payload)
            self.result_label.setText(f"Published test payload to {response.get('topic')}. Watch Devices / Graphs for ingestion.")
            self.refresh()
        except RuntimeError as exc:
            self.result_label.setText(f"MQTT publish failed: {exc}")

    def _test_modbus_connection(self) -> None:
        try:
            response = self.api_client.modbus_test_connection(self._modbus_connection_payload())
            self.modbus_result.setPlainText(json.dumps(response, indent=2))
            self.refresh()
        except RuntimeError as exc:
            self.modbus_result.setPlainText(f"Modbus TCP test failed:\n{exc}")

    def _read_modbus_registers(self) -> None:
        payload = self._modbus_connection_payload()
        payload.update(
            {
                "function": self.modbus_function_combo.currentText(),
                "address": self.modbus_address_input.value(),
                "quantity": self.modbus_quantity_input.value(),
            }
        )
        try:
            response = self.api_client.modbus_read_registers(payload)
            self.modbus_result.setPlainText(json.dumps(response, indent=2))
            self.refresh()
        except RuntimeError as exc:
            self.modbus_result.setPlainText(f"Modbus register read failed:\n{exc}")

    def _modbus_connection_payload(self) -> dict:
        return {
            "host": self.modbus_host_input.text().strip(),
            "port": self.modbus_port_input.value(),
            "unit_id": self.modbus_unit_input.value(),
            "timeout_ms": self.modbus_timeout_input.value(),
        }

    def _load_industrial_payload_template(self) -> None:
        if not hasattr(self, "industrial_payload_editor"):
            return
        protocol = self.industrial_protocol_combo.currentText() if hasattr(self, "industrial_protocol_combo") else "opcua"
        action = self.industrial_action_combo.currentText() if hasattr(self, "industrial_action_combo") else "test"
        templates = {
            ("opcua", "test"): {"endpoint_url": "opc.tcp://127.0.0.1:4840", "timeout_ms": 3000},
            ("opcua", "browse"): {
                "endpoint_url": "opc.tcp://127.0.0.1:4840",
                "node_id": "ns=0;i=85",
                "max_references": 50,
                "include_subtypes": True,
                "timeout_ms": 3000,
            },
            ("opcua", "read_node"): {
                "endpoint_url": "opc.tcp://127.0.0.1:4840",
                "node_id": "ns=0;i=2258",
                "attribute": "value",
                "timeout_ms": 3000,
            },
            ("bacnet", "test"): {"host": "127.0.0.1", "port": 47808, "timeout_ms": 3000, "expect_response": False},
            ("bacnet", "read_property"): {
                "host": "127.0.0.1",
                "port": 47808,
                "object_type": "analog_input",
                "object_instance": 1,
                "property": "present_value",
                "timeout_ms": 3000,
            },
            ("modbus_rtu", "test"): {
                "port": "COM3",
                "baud_rate": 9600,
                "data_bits": 8,
                "parity": "none",
                "stop_bits": 1,
                "timeout_ms": 3000,
            },
            ("modbus_rtu", "read_registers"): {
                "port": "COM3",
                "baud_rate": 9600,
                "data_bits": 8,
                "parity": "none",
                "stop_bits": 1,
                "unit_id": 1,
                "function": "holding_registers",
                "address": 0,
                "quantity": 2,
                "timeout_ms": 3000,
            },
            ("ads", "test"): {"host": "127.0.0.1", "port": 48898, "timeout_ms": 3000},
            ("ads", "read_symbol"): {
                "host": "127.0.0.1",
                "port": 48898,
                "net_id": "127.0.0.1.1.1",
                "ams_port": 851,
                "local_net_id": "auto",
                "local_port": 10500,
                "symbol": "MAIN.myVar",
                "timeout_ms": 5000,
            },
            ("odbc", "test"): {"dsn": "MySystemDsn", "connection_string": "", "timeout_ms": 3000},
            ("odbc", "query"): {
                "dsn": "MySystemDsn",
                "connection_string": "",
                "query": "SELECT TOP 100 * FROM MyTable",
                "max_rows": 100,
                "timeout_ms": 5000,
            },
        }
        self.industrial_payload_editor.setPlainText(json.dumps(templates.get((protocol, action), {}), indent=2))

    def _refresh_industrial_actions(self) -> None:
        if not hasattr(self, "industrial_action_combo"):
            return
        actions = {
            "opcua": ["test", "browse", "read_node"],
            "bacnet": ["test", "read_property"],
            "modbus_rtu": ["test", "read_registers"],
            "ads": ["test", "read_symbol"],
            "odbc": ["test", "query"],
        }
        protocol = self.industrial_protocol_combo.currentText()
        self.industrial_action_combo.blockSignals(True)
        self.industrial_action_combo.clear()
        self.industrial_action_combo.addItems(actions.get(protocol, ["test"]))
        self.industrial_action_combo.blockSignals(False)
        self._load_industrial_payload_template()

    def _run_industrial_action(self) -> None:
        protocol = self.industrial_protocol_combo.currentText()
        action = self.industrial_action_combo.currentText()
        try:
            payload = json.loads(self.industrial_payload_editor.toPlainText())
        except json.JSONDecodeError as exc:
            QMessageBox.warning(self, "Invalid JSON", str(exc))
            return

        try:
            if protocol == "opcua" and action == "test":
                response = self.api_client.opcua_test_connection(payload)
            elif protocol == "opcua" and action == "browse":
                response = self.api_client.opcua_browse(payload)
            elif protocol == "opcua" and action == "read_node":
                response = self.api_client.opcua_read_node(payload)
            elif protocol == "bacnet" and action == "test":
                response = self.api_client.bacnet_test_connection(payload)
            elif protocol == "bacnet" and action == "read_property":
                response = self.api_client.bacnet_read_property(payload)
            elif protocol == "modbus_rtu" and action == "test":
                response = self.api_client.modbus_rtu_test_connection(payload)
            elif protocol == "modbus_rtu" and action == "read_registers":
                response = self.api_client.modbus_rtu_read_registers(payload)
            elif protocol == "ads" and action == "test":
                response = self.api_client.ads_test_connection(payload)
            elif protocol == "ads" and action == "read_symbol":
                response = self.api_client.ads_read_symbol(payload)
            elif protocol == "odbc" and action == "test":
                response = self.api_client.odbc_test_connection(payload)
            elif protocol == "odbc" and action == "query":
                response = self.api_client.odbc_query(payload)
            else:
                raise RuntimeError(f"unsupported protocol/action: {protocol}/{action}")
            self.industrial_result.setPlainText(json.dumps(response, indent=2))
            self.refresh()
        except RuntimeError as exc:
            self.industrial_result.setPlainText(f"{protocol} {action} failed:\n{exc}")
