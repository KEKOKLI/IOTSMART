from __future__ import annotations

from collections import defaultdict
from datetime import datetime

from PySide6.QtCharts import (
    QAreaSeries,
    QBarCategoryAxis,
    QBarSeries,
    QBarSet,
    QChart,
    QChartView,
    QDateTimeAxis,
    QLineSeries,
    QPieSeries,
    QScatterSeries,
    QSplineSeries,
    QStackedBarSeries,
    QValueAxis,
)
from PySide6.QtCore import QDateTime, Qt
from PySide6.QtGui import QColor, QPainter, QPen


ACCENT = ["#1a73e8", "#34a853", "#fbbc04", "#ea4335", "#00acc1", "#a142f4"]
HEAT = ["#e8f0fe", "#d2e3fc", "#aecbfa", "#669df6", "#1a73e8"]
TEXT = QColor("#202124")
MUTED = QColor("#5f6368")
GRID = QColor("#eef0f1")


class ChartCanvas(QChartView):
    def __init__(self) -> None:
        super().__init__()
        self.setRenderHint(QPainter.Antialiasing)
        self._chart = QChart()
        self._chart.setBackgroundVisible(True)
        self._chart.setBackgroundBrush(QColor("#ffffff"))
        self._chart.setPlotAreaBackgroundVisible(True)
        self._chart.setPlotAreaBackgroundBrush(QColor("#ffffff"))
        self._chart.setDropShadowEnabled(False)
        self._chart.legend().setVisible(True)
        self._chart.legend().setLabelColor(MUTED)
        self._chart.setTitleBrush(TEXT)
        self.setChart(self._chart)
        self.show_message("Choose a device, metric, and chart.")

    def show_message(self, message: str) -> None:
        self._chart.removeAllSeries()
        self._chart.setTitle(message)
        for axis in list(self._chart.axes()):
            self._chart.removeAxis(axis)

    def set_data(
        self,
        chart_type: str,
        records: list[dict],
        comparison_records: list[dict] | None = None,
        title: str = "",
    ) -> None:
        chart_type = (chart_type or "line").lower()
        comparison_records = comparison_records or []
        if not records and not comparison_records:
            self.show_message("No telemetry data available for this chart.")
            return

        builders = {
            "line": self._build_line,
            "spline": self._build_spline,
            "area": self._build_area,
            "bar": self._build_bar,
            "stacked_bar": self._build_stacked_bar,
            "gauge": self._build_gauge,
            "heatmap": self._build_heatmap,
            "pie": self._build_pie,
            "donut": self._build_donut,
        }
        builder = builders.get(chart_type, self._build_line)
        self._chart.removeAllSeries()
        for axis in list(self._chart.axes()):
            self._chart.removeAxis(axis)
        self._chart.legend().setVisible(chart_type not in {"gauge", "heatmap"})
        self._chart.setTitle(title)
        builder(records, comparison_records)

    def _build_line(self, records: list[dict], _: list[dict]) -> None:
        series = QLineSeries()
        series.setName("Value")
        self._fill_time_series(series, records, ACCENT[0])
        self._attach_time_axes(series, records)

    def _build_spline(self, records: list[dict], _: list[dict]) -> None:
        series = QSplineSeries()
        series.setName("Trend")
        self._fill_time_series(series, records, ACCENT[1])
        self._attach_time_axes(series, records)

    def _build_area(self, records: list[dict], _: list[dict]) -> None:
        upper = QLineSeries()
        lower = QLineSeries()
        for index, record in enumerate(records):
            x_value, y_value = _record_point(record, index)
            upper.append(x_value, y_value)
            lower.append(x_value, 0)
        upper.setColor(QColor(ACCENT[0]))
        area = QAreaSeries(upper, lower)
        area.setName("Area")
        area.setPen(QPen(QColor(ACCENT[0]), 2))
        fill = QColor(ACCENT[0])
        fill.setAlpha(60)
        area.setBrush(fill)
        self._chart.addSeries(area)
        self._attach_time_axes(area, records)

    def _build_bar(self, records: list[dict], comparison_records: list[dict]) -> None:
        items = comparison_records or records[-10:]
        categories = []
        bar_set = QBarSet("Value")
        bar_set.setColor(QColor(ACCENT[2]))
        for item in items:
            categories.append(_label_for_record(item))
            bar_set.append(_to_float(item.get("value")))
        series = QBarSeries()
        series.append(bar_set)
        self._chart.addSeries(series)
        self._attach_category_axes(series, categories)

    def _build_stacked_bar(self, records: list[dict], comparison_records: list[dict]) -> None:
        items = comparison_records or records[-8:]
        values = [max(_to_float(item.get("value")), 0) for item in items]
        max_value = max(values) if values else 1
        target = max(max_value * 1.15, 1)

        actual = QBarSet("Value")
        reserve = QBarSet("Headroom")
        actual.setColor(QColor(ACCENT[0]))
        reserve.setColor(QColor("#e8f0fe"))
        categories = []
        for item, value in zip(items, values):
            categories.append(_label_for_record(item))
            actual.append(value)
            reserve.append(max(target - value, 0))

        series = QStackedBarSeries()
        series.append(actual)
        series.append(reserve)
        self._chart.addSeries(series)
        self._attach_category_axes(series, categories)

    def _build_gauge(self, records: list[dict], comparison_records: list[dict]) -> None:
        source = records or comparison_records
        latest = source[-1]
        values = [_to_float(item.get("value")) for item in source]
        value = _to_float(latest.get("value"))
        lower = min([0, *values])
        upper = max([100, *values])
        if upper <= lower:
            upper = lower + 1
        normalized = max(0.0, min(1.0, (value - lower) / (upper - lower)))

        series = QPieSeries()
        series.setHoleSize(0.62)
        series.setPieStartAngle(180)
        series.setPieEndAngle(0)
        current = series.append(f"{value:.2f}", normalized)
        current.setBrush(QColor(ACCENT[0]))
        current.setLabelVisible(True)
        current.setLabelColor(TEXT)
        rest = series.append("Remaining", max(1.0 - normalized, 0.001))
        rest.setBrush(QColor("#e8eaed"))
        rest.setLabelVisible(False)
        self._chart.addSeries(series)
        self._chart.setTitle(f"{self._chart.title()} | latest {value:.2f}")

    def _build_heatmap(self, records: list[dict], comparison_records: list[dict]) -> None:
        items = (records or comparison_records)[-96:]
        values = [_to_float(item.get("value")) for item in items]
        if not values:
            self.show_message("No data available for heatmap.")
            return
        low = min(values)
        high = max(values)
        buckets = []
        for color in HEAT:
            series = QScatterSeries()
            series.setColor(QColor(color))
            series.setMarkerSize(16)
            series.setName("")
            try:
                series.setMarkerShape(QScatterSeries.MarkerShapeRectangle)
            except AttributeError:
                pass
            buckets.append(series)

        for index, item in enumerate(items):
            value = _to_float(item.get("value"))
            bucket = _bucket(value, low, high, len(buckets))
            buckets[bucket].append(index % 24, index // 24)

        for series in buckets:
            if series.count() > 0:
                self._chart.addSeries(series)

        axis_x = QValueAxis()
        axis_x.setRange(-1, 24)
        axis_x.setLabelsVisible(False)
        axis_x.setGridLineVisible(False)
        axis_y = QValueAxis()
        axis_y.setRange(-1, max(4, len(items) // 24 + 1))
        axis_y.setLabelsVisible(False)
        axis_y.setGridLineVisible(False)
        self._chart.addAxis(axis_x, Qt.AlignBottom)
        self._chart.addAxis(axis_y, Qt.AlignLeft)
        for series in self._chart.series():
            series.attachAxis(axis_x)
            series.attachAxis(axis_y)

    def _build_pie(self, records: list[dict], comparison_records: list[dict]) -> None:
        self._build_pie_series(records, comparison_records, hole_size=0.0)

    def _build_donut(self, records: list[dict], comparison_records: list[dict]) -> None:
        self._build_pie_series(records, comparison_records, hole_size=0.48)

    def _build_pie_series(self, records: list[dict], comparison_records: list[dict], hole_size: float) -> None:
        items = comparison_records or records[-6:]
        if not items:
            self.show_message("No data available for this chart.")
            return
        series = QPieSeries()
        series.setHoleSize(hole_size)
        for index, item in enumerate(items):
            slice_item = series.append(_label_for_record(item), abs(_to_float(item.get("value"))))
            slice_item.setBrush(QColor(ACCENT[index % len(ACCENT)]))
            slice_item.setLabelVisible(True)
            slice_item.setLabelColor(TEXT)
        self._chart.addSeries(series)

    def _fill_time_series(self, series: QLineSeries | QSplineSeries, records: list[dict], color: str) -> None:
        for index, record in enumerate(records):
            x_value, y_value = _record_point(record, index)
            series.append(x_value, y_value)
        series.setPen(QPen(QColor(color), 2))
        self._chart.addSeries(series)

    def _attach_time_axes(self, series, records: list[dict]) -> None:
        axis_x = QDateTimeAxis()
        axis_x.setFormat("MM-dd HH:mm")
        axis_y = QValueAxis()
        self._style_axis(axis_x)
        self._style_axis(axis_y)
        self._chart.addAxis(axis_x, Qt.AlignBottom)
        self._chart.addAxis(axis_y, Qt.AlignLeft)
        series.attachAxis(axis_x)
        series.attachAxis(axis_y)
        axis_y.applyNiceNumbers()
        if records:
            first_ms, _ = _record_point(records[0], 0)
            last_ms, _ = _record_point(records[-1], len(records) - 1)
            if int(first_ms) == int(last_ms):
                last_ms += 60000
            axis_x.setRange(
                QDateTime.fromMSecsSinceEpoch(int(first_ms)),
                QDateTime.fromMSecsSinceEpoch(int(last_ms)),
            )

    def _attach_category_axes(self, series, categories: list[str]) -> None:
        axis_x = QBarCategoryAxis()
        axis_x.append(categories)
        axis_y = QValueAxis()
        self._style_axis(axis_x)
        self._style_axis(axis_y)
        self._chart.addAxis(axis_x, Qt.AlignBottom)
        self._chart.addAxis(axis_y, Qt.AlignLeft)
        series.attachAxis(axis_x)
        series.attachAxis(axis_y)
        axis_y.applyNiceNumbers()

    @staticmethod
    def _style_axis(axis) -> None:
        axis.setLabelsColor(MUTED)
        axis.setGridLineColor(GRID)
        axis.setLinePenColor(QColor("#dadce0"))


def build_comparison_records(metric: str, latest_records: list[dict]) -> list[dict]:
    filtered = [item for item in latest_records if item.get("metric") == metric]
    if filtered:
        return [
            {
                "device_id": item.get("device_id", "device"),
                "metric": item.get("metric", metric),
                "value": _to_float(item.get("value")),
            }
            for item in filtered
        ]

    grouped: dict[str, list[dict]] = defaultdict(list)
    for item in latest_records:
        grouped[item.get("metric", "value")].append(item)
    result: list[dict] = []
    for metric_name, items in grouped.items():
        if not items:
            continue
        result.append(
            {
                "device_id": metric_name,
                "metric": metric_name,
                "value": sum(_to_float(entry.get("value")) for entry in items) / len(items),
            }
        )
    return result


def _record_point(record: dict, index: int) -> tuple[float, float]:
    timestamp = record.get("timestamp")
    if isinstance(timestamp, str) and timestamp:
        try:
            parsed = datetime.fromisoformat(timestamp.replace("Z", "+00:00"))
            return float(int(parsed.timestamp() * 1000)), _to_float(record.get("value"))
        except (TypeError, ValueError):
            pass
    return float(index), _to_float(record.get("value"))


def _label_for_record(record: dict) -> str:
    timestamp = record.get("timestamp")
    if isinstance(timestamp, str) and timestamp:
        try:
            parsed = datetime.fromisoformat(timestamp.replace("Z", "+00:00"))
            return parsed.strftime("%H:%M")
        except (TypeError, ValueError):
            pass
    return str(record.get("device_id") or record.get("metric") or "value")


def _bucket(value: float, low: float, high: float, bucket_count: int) -> int:
    if high <= low:
        return bucket_count // 2
    normalized = (value - low) / (high - low)
    return max(0, min(bucket_count - 1, int(normalized * bucket_count)))


def _to_float(value) -> float:
    try:
        if value is None:
            return 0.0
        return float(value)
    except (TypeError, ValueError):
        return 0.0
