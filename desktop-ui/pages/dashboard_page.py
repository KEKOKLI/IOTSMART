from __future__ import annotations

from collections import defaultdict
from typing import Callable

from PySide6.QtWidgets import QComboBox, QFrame, QGridLayout, QHBoxLayout, QLabel, QPushButton, QVBoxLayout, QWidget

from widgets.graph_panel import GraphPanel
from widgets.stat_card import StatCard


class DashboardPage(QWidget):
    def __init__(self, api_client) -> None:
        super().__init__()
        self.api_client = api_client
        self.navigation_handler: Callable[[str], None] | None = None
        self.devices_cache: list[dict] = []

        layout = QVBoxLayout(self)
        layout.setSpacing(18)

        layout.addWidget(self._build_hero())
        layout.addWidget(self._build_options_panel())

        self.health_card = StatCard("System", "Checking", "Waiting for backend")
        self.online_card = StatCard("Online sensors", "0", "Live devices")
        self.offline_card = StatCard("Needs attention", "0", "Offline or stale")
        self.latest_card = StatCard("Latest signals", "--", "No latest metrics yet")

        grid = QGridLayout()
        grid.setSpacing(14)
        grid.addWidget(self.health_card, 0, 0)
        grid.addWidget(self.online_card, 0, 1)
        grid.addWidget(self.offline_card, 0, 2)
        grid.addWidget(self.latest_card, 0, 3)

        lower = QHBoxLayout()
        lower.setSpacing(14)
        self.chart_panel = GraphPanel(api_client, title="Temperature trend")
        self.attention_panel = self._build_attention_panel()
        lower.addWidget(self.chart_panel, 3)
        lower.addWidget(self.attention_panel, 1)

        layout.addLayout(grid)
        layout.addLayout(lower, 1)

    def set_navigation_handler(self, handler: Callable[[str], None]) -> None:
        self.navigation_handler = handler

    def _build_hero(self) -> QWidget:
        panel = QFrame()
        panel.setProperty("hero", True)
        layout = QHBoxLayout(panel)
        layout.setContentsMargins(22, 20, 22, 20)
        layout.setSpacing(18)

        text_block = QVBoxLayout()
        self.hero_title = QLabel("Your sensor site at a glance")
        self.hero_title.setObjectName("PageTitle")
        self.hero_subtitle = QLabel("Live local telemetry, clear device status, and quick access to MQTT testing.")
        self.hero_subtitle.setProperty("muted", True)
        text_block.addWidget(self.hero_title)
        text_block.addWidget(self.hero_subtitle)

        actions = QHBoxLayout()
        devices_button = QPushButton("Manage Devices")
        devices_button.setProperty("accent", True)
        devices_button.clicked.connect(lambda: self._go("Devices"))
        graphs_button = QPushButton("Build Graph")
        graphs_button.setProperty("secondary", True)
        graphs_button.clicked.connect(lambda: self._go("Visuals"))
        mqtt_button = QPushButton("Test MQTT")
        mqtt_button.clicked.connect(lambda: self._go("Protocols"))
        actions.addWidget(devices_button)
        actions.addWidget(graphs_button)
        actions.addWidget(mqtt_button)
        actions.addStretch(1)
        text_block.addLayout(actions)

        self.hero_pill = QLabel("Checking backend")
        self.hero_pill.setProperty("pill", True)
        self.hero_pill.setProperty("pillState", "warn")

        layout.addLayout(text_block, 1)
        layout.addWidget(self.hero_pill)
        return panel

    def _build_options_panel(self) -> QWidget:
        panel = QFrame()
        panel.setProperty("card", True)
        layout = QVBoxLayout(panel)
        layout.setContentsMargins(18, 16, 18, 16)
        layout.setSpacing(10)

        title = QLabel("Dashboard Options")
        title.setObjectName("SectionTitle")
        hint = QLabel("Choose the exact chart you want on the front page. This does not change stored telemetry.")
        hint.setProperty("muted", True)

        self.dashboard_device_combo = QComboBox()
        self.dashboard_metric_combo = QComboBox()
        self.dashboard_chart_combo = QComboBox()
        self.dashboard_range_combo = QComboBox()
        self.dashboard_chart_combo.addItems(["spline", "line", "area", "bar", "stacked_bar", "gauge", "heatmap"])
        self.dashboard_range_combo.addItems(["1h", "6h", "24h", "7d"])
        self.dashboard_device_combo.currentTextChanged.connect(self._refresh_metric_options)

        controls = QGridLayout()
        controls.setSpacing(10)

        apply_button = QPushButton("Update Dashboard")
        apply_button.setProperty("secondary", True)
        apply_button.clicked.connect(self._refresh_dashboard_chart)

        layout.addWidget(title)
        layout.addWidget(hint)
        controls.addWidget(QLabel("Device"), 0, 0)
        controls.addWidget(self.dashboard_device_combo, 1, 0)
        controls.addWidget(QLabel("Metric"), 0, 1)
        controls.addWidget(self.dashboard_metric_combo, 1, 1)
        controls.addWidget(QLabel("Chart"), 0, 2)
        controls.addWidget(self.dashboard_chart_combo, 1, 2)
        controls.addWidget(QLabel("Range"), 0, 3)
        controls.addWidget(self.dashboard_range_combo, 1, 3)
        controls.addWidget(apply_button, 1, 4)
        layout.addLayout(controls)
        return panel

    def _build_attention_panel(self) -> QWidget:
        panel = QFrame()
        panel.setProperty("notice", True)
        layout = QVBoxLayout(panel)
        layout.setContentsMargins(18, 16, 18, 16)
        layout.setSpacing(10)

        title = QLabel("What needs attention")
        title.setObjectName("SectionTitle")
        self.attention_label = QLabel("No issues loaded yet.")
        self.attention_label.setWordWrap(True)
        self.attention_label.setProperty("muted", True)
        self.next_step_label = QLabel("Tip: Use Test MQTT to verify broker flow before adding real sensors.")
        self.next_step_label.setWordWrap(True)
        self.next_step_label.setProperty("muted", True)
        action_row = QHBoxLayout()
        devices_button = QPushButton("Devices")
        devices_button.setProperty("secondary", True)
        devices_button.clicked.connect(lambda: self._go("Devices"))
        protocols_button = QPushButton("MQTT Test")
        protocols_button.setProperty("secondary", True)
        protocols_button.clicked.connect(lambda: self._go("Protocols"))
        logs_button = QPushButton("Logs")
        logs_button.clicked.connect(lambda: self._go("Logs"))
        action_row.addWidget(devices_button)
        action_row.addWidget(protocols_button)
        action_row.addWidget(logs_button)
        action_row.addStretch(1)

        layout.addWidget(title)
        layout.addWidget(self.attention_label)
        layout.addSpacing(8)
        layout.addWidget(self.next_step_label)
        layout.addLayout(action_row)
        layout.addStretch(1)
        return panel

    def refresh(self) -> None:
        try:
            health = self.api_client.health()
            latest = self.api_client.latest_telemetry(limit=24)
        except RuntimeError as exc:
            self.hero_title.setText("Backend is not reachable")
            self.hero_subtitle.setText("Start the Go backend first, then this dashboard will fill itself.")
            self.hero_pill.setText("Offline")
            self._set_pill_state(self.hero_pill, "bad")
            self.health_card.update_values("Offline", str(exc))
            self.attention_label.setText(str(exc))
            return

        self._refresh_option_sources()

        online = _to_int(health.get("online_devices"))
        offline = _to_int(health.get("offline_devices"))
        status = str(health.get("status") or "unknown")

        if status == "ok" and offline == 0:
            self.hero_title.setText("Everything looks healthy")
            self.hero_subtitle.setText("Sensors are reporting normally. You can review live charts or export telemetry any time.")
            self.hero_pill.setText("All clear")
            self._set_pill_state(self.hero_pill, "ok")
        elif status == "ok":
            self.hero_title.setText("Some sensors need a look")
            self.hero_subtitle.setText("The backend is running, but one or more devices have stale or missing readings.")
            self.hero_pill.setText("Attention needed")
            self._set_pill_state(self.hero_pill, "warn")
        else:
            self.hero_title.setText("System status needs checking")
            self.hero_subtitle.setText("Review backend health and protocol connector status before trusting the latest readings.")
            self.hero_pill.setText("Check system")
            self._set_pill_state(self.hero_pill, "bad")

        self.health_card.update_values(
            status.upper(),
            f"DB {health.get('db', 'unknown')} | MQTT {health.get('mqtt', 'unknown')}",
        )
        self.online_card.update_values(
            str(online),
            f"Workers {health.get('workers', 0)} | Uptime {health.get('uptime', 'n/a')}",
        )
        self.offline_card.update_values(
            str(offline),
            f"Last ingest {health.get('last_ingest_at', 'n/a')}",
        )

        grouped = defaultdict(list)
        for item in latest:
            grouped[item.get("metric", "value")].append(item)

        snippets = []
        for metric, items in grouped.items():
            if not items:
                continue
            sample = items[0]
            snippets.append(f"{metric}: {_to_float(sample.get('value')):.2f}{sample.get('unit') or ''}")
        self.latest_card.update_values(str(len(grouped)) if grouped else "--", " | ".join(snippets[:3]) or "No active signals")

        if offline > 0:
            self.attention_label.setText(
                f"{offline} sensor(s) look offline. Open Devices to check location, protocol, and last-seen data."
            )
            self.next_step_label.setText("Next step: confirm power/network first, then test MQTT or HTTP ingest.")
        elif not latest:
            self.attention_label.setText("Backend is healthy, but no telemetry is available yet.")
            self.next_step_label.setText("Next step: publish a test MQTT payload or enable the simulator.")
        else:
            self.attention_label.setText("No urgent issues. Latest telemetry is flowing into the local database.")
            self.next_step_label.setText("Next step: save graph presets for the metrics you monitor every day.")

        self._refresh_dashboard_chart()

    def _refresh_option_sources(self) -> None:
        current_device = self.dashboard_device_combo.currentText()
        try:
            self.devices_cache = self.api_client.devices()
        except RuntimeError:
            self.devices_cache = []

        self.dashboard_device_combo.blockSignals(True)
        self.dashboard_device_combo.clear()
        self.dashboard_device_combo.addItem("all")
        self.dashboard_device_combo.addItem("sim_001")
        for device in self.devices_cache:
            device_id = device.get("id", "")
            if device_id and self.dashboard_device_combo.findText(device_id) < 0:
                self.dashboard_device_combo.addItem(device_id)
        self._set_combo(self.dashboard_device_combo, current_device or "sim_001")
        self.dashboard_device_combo.blockSignals(False)
        self._refresh_metric_options()

    def _refresh_metric_options(self) -> None:
        if not hasattr(self, "dashboard_metric_combo"):
            return
        current_metric = self.dashboard_metric_combo.currentText()
        current_device = self.dashboard_device_combo.currentText()
        metrics = []
        for device in self.devices_cache:
            if current_device not in ("", "all") and device.get("id") != current_device:
                continue
            metrics.extend(metric.get("metric", "") for metric in device.get("metrics", []))
        metrics = sorted({metric for metric in metrics if metric}) or ["temperature", "humidity", "voltage", "current"]

        self.dashboard_metric_combo.clear()
        self.dashboard_metric_combo.addItems(metrics)
        self._set_combo(self.dashboard_metric_combo, current_metric or "temperature")

    def _refresh_dashboard_chart(self) -> None:
        device_id = self.dashboard_device_combo.currentText() or "sim_001"
        metric = self.dashboard_metric_combo.currentText() or "temperature"
        chart_type = self.dashboard_chart_combo.currentText() or "spline"
        time_range = self.dashboard_range_combo.currentText() or "1h"
        self.chart_panel.refresh(device_id=device_id, metric=metric, chart_type=chart_type, time_range=time_range)

    def _go(self, page_name: str) -> None:
        if self.navigation_handler:
            self.navigation_handler(page_name)

    @staticmethod
    def _set_combo(combo: QComboBox, value: str) -> None:
        index = combo.findText(value)
        if index >= 0:
            combo.setCurrentIndex(index)

    @staticmethod
    def _set_pill_state(label: QLabel, state: str) -> None:
        label.setProperty("pillState", state)
        label.style().unpolish(label)
        label.style().polish(label)


def _to_float(value) -> float:
    try:
        if value is None:
            return 0.0
        return float(value)
    except (TypeError, ValueError):
        return 0.0


def _to_int(value) -> int:
    try:
        if value is None:
            return 0
        return int(value)
    except (TypeError, ValueError):
        return 0
