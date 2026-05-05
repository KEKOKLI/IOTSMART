from datetime import datetime

from PySide6.QtWidgets import QLabel, QVBoxLayout, QWidget

try:
    import pyqtgraph as pg
except ImportError:  # pragma: no cover - optional dependency for local desktop only
    pg = None


class LineChartWidget(QWidget):
    def __init__(self) -> None:
        super().__init__()
        layout = QVBoxLayout(self)
        self._message = QLabel("Waiting for data...")
        self._plot = None

        if pg is None:
            layout.addWidget(self._message)
            return

        self._plot = pg.PlotWidget()
        self._plot.showGrid(x=True, y=True, alpha=0.2)
        self._plot.setBackground("#fffdf9")
        layout.addWidget(self._plot)

    def set_records(self, records: list[dict]) -> None:
        if self._plot is None:
            self.show_message(f"{len(records)} records ready, install pyqtgraph to view chart.")
            return

        self._plot.clear()
        if not records:
            self.show_message("No telemetry data yet.")
            return

        xs = []
        ys = []
        for index, record in enumerate(records):
            timestamp = record.get("timestamp")
            if timestamp:
                try:
                    datetime.fromisoformat(timestamp.replace("Z", "+00:00"))
                except ValueError:
                    pass
            xs.append(index)
            ys.append(float(record.get("value", 0)))
        self._plot.plot(xs, ys, pen=pg.mkPen("#d97d54", width=2))

    def show_message(self, text: str) -> None:
        self._message.setText(text)
        if self._plot is None:
            return
        self._plot.clear()
