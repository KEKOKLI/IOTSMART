from __future__ import annotations

from datetime import datetime, timedelta, timezone

from PySide6.QtWidgets import QFrame, QLabel, QVBoxLayout

from charts.chart_canvas import ChartCanvas, build_comparison_records


class GraphPanel(QFrame):
    def __init__(self, api_client, title: str = "Telemetry Studio") -> None:
        super().__init__()
        self.api_client = api_client
        self.setProperty("card", True)

        layout = QVBoxLayout(self)
        self.title_label = QLabel(title)
        self.title_label.setProperty("accent", True)
        self.subtitle_label = QLabel("Pick a metric and turn live telemetry into a readable daily view.")
        self.subtitle_label.setProperty("muted", True)
        self.chart = ChartCanvas()
        layout.addWidget(self.title_label)
        layout.addWidget(self.subtitle_label)
        layout.addWidget(self.chart, 1)

    def refresh(
        self,
        device_id: str,
        metric: str,
        chart_type: str = "line",
        time_range: str = "1h",
    ) -> None:
        if not metric:
            self.chart.show_message("Choose a metric to visualize.")
            return

        params = {"metric": metric, "limit": 240}
        if device_id and device_id != "all":
            params["device_id"] = device_id
        from_ts = _range_to_from_timestamp(time_range)
        if from_ts:
            params["from"] = from_ts

        try:
            records = self.api_client.query_telemetry(**params)
            records = list(reversed(records or []))
            latest = self.api_client.latest_telemetry(limit=200)
        except RuntimeError as exc:
            self.chart.show_message(str(exc))
            return

        comparison = build_comparison_records(metric, latest or [])
        title = f"{metric} | {chart_type.upper()} | {time_range}"
        self.chart.set_data(chart_type, records, comparison, title=title)


def _range_to_from_timestamp(value: str) -> str | None:
    now = datetime.now(timezone.utc)
    options = {
        "1h": timedelta(hours=1),
        "6h": timedelta(hours=6),
        "24h": timedelta(hours=24),
        "7d": timedelta(days=7),
    }
    delta = options.get(value)
    if delta is None:
        return None
    return (now - delta).isoformat().replace("+00:00", "Z")
