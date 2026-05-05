from __future__ import annotations

from datetime import datetime, timedelta, timezone

from PySide6.QtWidgets import (
    QFileDialog,
    QFormLayout,
    QFrame,
    QHBoxLayout,
    QInputDialog,
    QLabel,
    QListWidget,
    QMessageBox,
    QPushButton,
    QComboBox,
    QVBoxLayout,
    QWidget,
)

from widgets.graph_panel import GraphPanel


class GraphBuilderPage(QWidget):
    def __init__(self, api_client) -> None:
        super().__init__()
        self.api_client = api_client
        self.devices_cache: list[dict] = []
        self.presets_cache: list[dict] = []
        self.active_preset_id: str | None = None

        layout = QHBoxLayout(self)
        layout.addWidget(self._build_control_panel(), 0)
        layout.addWidget(self._build_chart_panel(), 1)

    def _build_control_panel(self) -> QWidget:
        panel = QFrame()
        panel.setProperty("card", True)
        layout = QVBoxLayout(panel)

        title = QLabel("Visual Builder")
        title.setProperty("accent", True)
        hint = QLabel("Choose what you want to understand, then switch between trend, comparison, gauge, heatmap, and shareable CSV views.")
        hint.setProperty("muted", True)

        form = QFormLayout()
        self.device_combo = QComboBox()
        self.metric_combo = QComboBox()
        self.chart_combo = QComboBox()
        self.chart_combo.addItems(["line", "spline", "area", "bar", "stacked_bar", "gauge", "heatmap", "pie", "donut"])
        self.range_combo = QComboBox()
        self.range_combo.addItems(["1h", "6h", "24h", "7d"])

        form.addRow("Device", self.device_combo)
        form.addRow("Metric", self.metric_combo)
        form.addRow("Chart", self.chart_combo)
        form.addRow("Time range", self.range_combo)

        self.device_combo.currentTextChanged.connect(self._refresh_metric_options)

        toolbar = QHBoxLayout()
        refresh_button = QPushButton("Render")
        refresh_button.setProperty("accent", True)
        refresh_button.clicked.connect(self._render_chart)
        preset_button = QPushButton("Save Preset")
        preset_button.clicked.connect(self._save_preset)
        delete_button = QPushButton("Delete Preset")
        delete_button.setProperty("danger", True)
        delete_button.clicked.connect(self._delete_preset)
        export_button = QPushButton("Export CSV")
        export_button.clicked.connect(self._export_csv)
        toolbar.addWidget(refresh_button)
        toolbar.addWidget(preset_button)
        toolbar.addWidget(delete_button)
        toolbar.addWidget(export_button)

        self.preset_list = QListWidget()
        self.preset_list.itemSelectionChanged.connect(self._load_selected_preset)

        layout.addWidget(title)
        layout.addWidget(hint)
        layout.addLayout(form)
        layout.addLayout(toolbar)
        layout.addWidget(QLabel("Saved Presets"))
        layout.addWidget(self.preset_list, 1)
        return panel

    def _build_chart_panel(self) -> QWidget:
        panel = QFrame()
        panel.setProperty("glass", True)
        layout = QVBoxLayout(panel)
        self.graph_panel = GraphPanel(self.api_client, title="Live visual preview")
        self.status_label = QLabel("Ready.")
        self.status_label.setProperty("muted", True)
        layout.addWidget(self.graph_panel, 1)
        layout.addWidget(self.status_label)
        return panel

    def refresh(self) -> None:
        try:
            self.devices_cache = self.api_client.devices()
            self.presets_cache = self.api_client.graph_presets()
        except RuntimeError as exc:
            self.status_label.setText(str(exc))
            return

        self._refresh_device_options()
        self._refresh_metric_options()
        self._refresh_preset_list()
        self._render_chart()

    def _refresh_device_options(self) -> None:
        current = self.device_combo.currentText()
        self.device_combo.blockSignals(True)
        self.device_combo.clear()
        self.device_combo.addItem("all")
        for device in self.devices_cache:
            self.device_combo.addItem(device.get("id", ""))
        if current:
            index = self.device_combo.findText(current)
            if index >= 0:
                self.device_combo.setCurrentIndex(index)
        self.device_combo.blockSignals(False)

    def _refresh_metric_options(self) -> None:
        current_device = self.device_combo.currentText()
        current_metric = self.metric_combo.currentText()
        metrics = []
        for device in self.devices_cache:
            if current_device not in ("", "all") and device.get("id") != current_device:
                continue
            metrics.extend(metric.get("metric", "") for metric in device.get("metrics", []))
        metrics = sorted({metric for metric in metrics if metric})

        self.metric_combo.clear()
        self.metric_combo.addItems(metrics)
        if current_metric:
            index = self.metric_combo.findText(current_metric)
            if index >= 0:
                self.metric_combo.setCurrentIndex(index)

    def _refresh_preset_list(self) -> None:
        self.preset_list.clear()
        for preset in self.presets_cache:
            self.preset_list.addItem(f"{preset.get('name')} | {preset.get('graph_type')} | {preset.get('metric')}")

    def _render_chart(self) -> None:
        device_id = self.device_combo.currentText()
        metric = self.metric_combo.currentText()
        chart_type = self.chart_combo.currentText()
        time_range = self.range_combo.currentText()

        if not metric:
            self.status_label.setText("Choose a metric to render.")
            return

        from_ts = _time_range_start(time_range)
        try:
            params = {"metric": metric, "limit": 240}
            if device_id and device_id != "all":
                params["device_id"] = device_id
            if from_ts:
                params["from"] = from_ts
            records = self.api_client.query_telemetry(**params)
            latest = self.api_client.latest_telemetry(limit=200)
        except RuntimeError as exc:
            self.status_label.setText(str(exc))
            return

        records = list(reversed(records))
        comparison = [item for item in latest if item.get("metric") == metric]
        title = f"{metric} | {chart_type.upper()} | {time_range}"
        self.graph_panel.chart.set_data(chart_type, records, comparison_records=comparison, title=title)
        self.status_label.setText(f"Rendered {len(records)} points.")

    def _save_preset(self) -> None:
        metric = self.metric_combo.currentText()
        if not metric:
            QMessageBox.warning(self, "Missing metric", "Select a metric before saving a preset.")
            return
        name, ok = QInputDialog.getText(self, "Preset name", "Name")
        if not ok or not name.strip():
            return
        payload = {
            "name": name.strip(),
            "device_id": self.device_combo.currentText(),
            "metric": metric,
            "graph_type": self.chart_combo.currentText(),
            "time_range": self.range_combo.currentText(),
        }
        try:
            if self.active_preset_id:
                self.api_client.update_graph_preset(self.active_preset_id, payload)
            else:
                self.api_client.create_graph_preset(payload)
            self.refresh()
            self.status_label.setText("Preset saved.")
        except RuntimeError as exc:
            QMessageBox.warning(self, "Preset error", str(exc))

    def _delete_preset(self) -> None:
        if not self.active_preset_id:
            QMessageBox.information(self, "No preset selected", "Choose a preset first.")
            return
        try:
            self.api_client.delete_graph_preset(self.active_preset_id)
            self.active_preset_id = None
            self.refresh()
            self.status_label.setText("Preset deleted.")
        except RuntimeError as exc:
            QMessageBox.warning(self, "Delete failed", str(exc))

    def _load_selected_preset(self) -> None:
        row = self.preset_list.currentRow()
        if row < 0 or row >= len(self.presets_cache):
            return
        preset = self.presets_cache[row]
        self.active_preset_id = preset.get("id")
        self._set_combo_value(self.device_combo, preset.get("device_id", "all"))
        self._refresh_metric_options()
        self._set_combo_value(self.metric_combo, preset.get("metric", ""))
        self._set_combo_value(self.chart_combo, preset.get("graph_type", "line"))
        self._set_combo_value(self.range_combo, preset.get("time_range", "1h"))
        self._render_chart()

    def _export_csv(self) -> None:
        metric = self.metric_combo.currentText()
        if not metric:
            QMessageBox.warning(self, "Missing metric", "Select a metric before exporting.")
            return
        destination, _ = QFileDialog.getSaveFileName(self, "Export telemetry CSV", "", "CSV Files (*.csv)")
        if not destination:
            return
        params = {"metric": metric, "limit": 5000}
        device_id = self.device_combo.currentText()
        if device_id and device_id != "all":
            params["device_id"] = device_id
        from_ts = _time_range_start(self.range_combo.currentText())
        if from_ts:
            params["from"] = from_ts
        try:
            self.api_client.export_csv(destination, **params)
            self.status_label.setText(f"Exported CSV to {destination}")
        except RuntimeError as exc:
            QMessageBox.warning(self, "Export failed", str(exc))

    @staticmethod
    def _set_combo_value(combo: QComboBox, value: str) -> None:
        index = combo.findText(value)
        if index >= 0:
            combo.setCurrentIndex(index)


def _time_range_start(time_range: str) -> str | None:
    now = datetime.now(timezone.utc)
    mapping = {
        "1h": timedelta(hours=1),
        "6h": timedelta(hours=6),
        "24h": timedelta(hours=24),
        "7d": timedelta(days=7),
    }
    delta = mapping.get(time_range)
    if delta is None:
        return None
    return (now - delta).isoformat().replace("+00:00", "Z")
