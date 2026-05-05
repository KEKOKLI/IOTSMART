from __future__ import annotations

from PySide6.QtCore import Qt
from PySide6.QtWidgets import (
    QCheckBox,
    QFormLayout,
    QFrame,
    QHBoxLayout,
    QLabel,
    QLineEdit,
    QMessageBox,
    QPushButton,
    QSplitter,
    QTableWidget,
    QTableWidgetItem,
    QVBoxLayout,
    QWidget,
    QComboBox,
)


class DevicesPage(QWidget):
    def __init__(self, api_client) -> None:
        super().__init__()
        self.api_client = api_client
        self.devices_cache: list[dict] = []
        self.visible_devices: list[dict] = []
        self.selected_device_id: str | None = None

        layout = QVBoxLayout(self)
        splitter = QSplitter(Qt.Horizontal)
        layout.addWidget(splitter)

        splitter.addWidget(self._build_device_list_panel())
        splitter.addWidget(self._build_editor_panel())
        splitter.setSizes([580, 520])

    def _build_device_list_panel(self) -> QWidget:
        panel = QFrame()
        panel.setProperty("card", True)
        layout = QVBoxLayout(panel)

        title = QLabel("Device Fleet")
        title.setProperty("accent", True)
        hint = QLabel("Create or edit devices with metric definitions for MQTT, HTTP ingest, or future protocol workers.")
        hint.setProperty("muted", True)

        self.table = QTableWidget(0, 6)
        self.table.setHorizontalHeaderLabels(["ID", "Name", "Protocol", "Location", "Enabled", "Metrics"])
        self.table.itemSelectionChanged.connect(self._load_selected_device)

        action_row = QHBoxLayout()
        self.search_input = QLineEdit()
        self.search_input.setPlaceholderText("Search by ID, name, protocol, or location")
        self.search_input.textChanged.connect(self._populate_table)
        refresh_button = QPushButton("Refresh")
        refresh_button.clicked.connect(self.refresh)
        new_button = QPushButton("New Device")
        new_button.setProperty("accent", True)
        new_button.clicked.connect(self._reset_form)
        action_row.addWidget(self.search_input, 1)
        action_row.addWidget(refresh_button)
        action_row.addWidget(new_button)

        layout.addWidget(title)
        layout.addWidget(hint)
        layout.addLayout(action_row)
        layout.addWidget(self.table, 1)
        return panel

    def _build_editor_panel(self) -> QWidget:
        panel = QFrame()
        panel.setProperty("glass", True)
        layout = QVBoxLayout(panel)

        title = QLabel("Device Settings")
        title.setProperty("accent", True)
        self.status_label = QLabel("Select a device or create a new one.")
        self.status_label.setProperty("muted", True)

        form = QFormLayout()
        self.id_input = QLineEdit()
        self.project_input = QLineEdit("default-project")
        self.name_input = QLineEdit()
        self.protocol_input = QComboBox()
        self.protocol_input.addItems(["mqtt", "http_ingest", "modbus", "simulator"])
        self.location_input = QLineEdit()
        self.enabled_checkbox = QCheckBox("Enabled")
        self.enabled_checkbox.setChecked(True)

        form.addRow("Device ID", self.id_input)
        form.addRow("Project", self.project_input)
        form.addRow("Display Name", self.name_input)
        form.addRow("Protocol", self.protocol_input)
        form.addRow("Location", self.location_input)
        form.addRow("", self.enabled_checkbox)

        self.metrics_table = QTableWidget(0, 4)
        self.metrics_table.setHorizontalHeaderLabels(["Metric", "Unit", "Min", "Max"])

        metric_actions = QHBoxLayout()
        add_metric_button = QPushButton("Add Metric")
        add_metric_button.clicked.connect(self._add_metric_row)
        remove_metric_button = QPushButton("Remove Metric")
        remove_metric_button.clicked.connect(self._remove_metric_row)
        metric_actions.addWidget(add_metric_button)
        metric_actions.addWidget(remove_metric_button)
        metric_actions.addStretch(1)

        button_row = QHBoxLayout()
        save_button = QPushButton("Save Device")
        save_button.setProperty("accent", True)
        save_button.clicked.connect(self._save_device)
        delete_button = QPushButton("Delete Device")
        delete_button.setProperty("danger", True)
        delete_button.clicked.connect(self._delete_device)
        button_row.addWidget(save_button)
        button_row.addWidget(delete_button)
        button_row.addStretch(1)

        layout.addWidget(title)
        layout.addWidget(self.status_label)
        layout.addLayout(form)
        layout.addWidget(QLabel("Metrics"))
        layout.addWidget(self.metrics_table, 1)
        layout.addLayout(metric_actions)
        layout.addLayout(button_row)
        return panel

    def refresh(self) -> None:
        try:
            self.devices_cache = self.api_client.devices()
        except RuntimeError as exc:
            self.status_label.setText(str(exc))
            return

        self._populate_table()

    def _populate_table(self) -> None:
        query = self.search_input.text().strip().lower() if hasattr(self, "search_input") else ""
        self.visible_devices = []
        for device in self.devices_cache:
            metrics = ", ".join(metric.get("metric", "") for metric in device.get("metrics", []))
            haystack = " ".join(
                [
                    device.get("id", ""),
                    device.get("name", ""),
                    device.get("protocol", ""),
                    device.get("location", ""),
                    metrics,
                ]
            ).lower()
            if query and query not in haystack:
                continue
            self.visible_devices.append(device)

        self.table.setRowCount(len(self.visible_devices))
        for row, device in enumerate(self.visible_devices):
            metrics = ", ".join(metric.get("metric", "") for metric in device.get("metrics", []))
            values = [
                device.get("id", ""),
                device.get("name", ""),
                device.get("protocol", ""),
                device.get("location", ""),
                str(device.get("enabled", False)),
                metrics,
            ]
            for column, value in enumerate(values):
                self.table.setItem(row, column, QTableWidgetItem(value))

    def _load_selected_device(self) -> None:
        row = self.table.currentRow()
        if row < 0 or row >= len(self.visible_devices):
            return
        device = self.visible_devices[row]
        self.selected_device_id = device.get("id")
        self.status_label.setText(f"Editing {self.selected_device_id}")
        self.id_input.setText(device.get("id", ""))
        self.id_input.setEnabled(False)
        self.project_input.setText(device.get("project_id", "default-project"))
        self.name_input.setText(device.get("name", ""))
        self.protocol_input.setCurrentText(device.get("protocol", "mqtt"))
        self.location_input.setText(device.get("location", ""))
        self.enabled_checkbox.setChecked(bool(device.get("enabled", True)))

        self.metrics_table.setRowCount(0)
        for metric in device.get("metrics", []):
            self._add_metric_row(
                metric.get("metric", ""),
                metric.get("unit", ""),
                metric.get("min_value"),
                metric.get("max_value"),
            )

    def _reset_form(self) -> None:
        self.selected_device_id = None
        self.status_label.setText("Creating a new device.")
        self.id_input.setEnabled(True)
        self.id_input.clear()
        self.project_input.setText("default-project")
        self.name_input.clear()
        self.protocol_input.setCurrentText("mqtt")
        self.location_input.clear()
        self.enabled_checkbox.setChecked(True)
        self.metrics_table.setRowCount(0)
        self._add_metric_row("temperature", "C", None, None)

    def _add_metric_row(self, metric: str = "", unit: str = "", minimum=None, maximum=None) -> None:
        row = self.metrics_table.rowCount()
        self.metrics_table.insertRow(row)
        values = [metric, unit, "" if minimum is None else str(minimum), "" if maximum is None else str(maximum)]
        for column, value in enumerate(values):
            self.metrics_table.setItem(row, column, QTableWidgetItem(value))

    def _remove_metric_row(self) -> None:
        row = self.metrics_table.currentRow()
        if row >= 0:
            self.metrics_table.removeRow(row)

    def _save_device(self) -> None:
        payload = {
            "id": self.id_input.text().strip(),
            "project_id": self.project_input.text().strip(),
            "name": self.name_input.text().strip(),
            "protocol": self.protocol_input.currentText(),
            "location": self.location_input.text().strip(),
            "enabled": self.enabled_checkbox.isChecked(),
            "metrics": self._collect_metrics(),
        }
        if not payload["id"]:
            QMessageBox.warning(self, "Missing ID", "Device ID is required.")
            return

        try:
            if self.selected_device_id:
                self.api_client.update_device(self.selected_device_id, payload)
            else:
                self.api_client.create_device(payload)
            self.refresh()
            self.status_label.setText("Device saved.")
        except RuntimeError as exc:
            QMessageBox.warning(self, "Save failed", str(exc))

    def _delete_device(self) -> None:
        if not self.selected_device_id:
            QMessageBox.information(self, "Nothing selected", "Choose a saved device to delete.")
            return
        try:
            self.api_client.delete_device(self.selected_device_id)
            self.refresh()
            self._reset_form()
            self.status_label.setText("Device deleted.")
        except RuntimeError as exc:
            QMessageBox.warning(self, "Delete failed", str(exc))

    def _collect_metrics(self) -> list[dict]:
        metrics = []
        for row in range(self.metrics_table.rowCount()):
            metric = _cell_text(self.metrics_table, row, 0)
            unit = _cell_text(self.metrics_table, row, 1)
            minimum = _to_float(_cell_text(self.metrics_table, row, 2))
            maximum = _to_float(_cell_text(self.metrics_table, row, 3))
            if not metric:
                continue
            payload = {"metric": metric, "unit": unit, "data_type": "float"}
            if minimum is not None:
                payload["min_value"] = minimum
            if maximum is not None:
                payload["max_value"] = maximum
            metrics.append(payload)
        return metrics


def _cell_text(table: QTableWidget, row: int, column: int) -> str:
    item = table.item(row, column)
    return item.text().strip() if item is not None else ""


def _to_float(value: str):
    if not value:
        return None
    try:
        return float(value)
    except ValueError:
        return None
