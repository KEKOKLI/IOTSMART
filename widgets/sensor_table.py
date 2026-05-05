from PySide6.QtWidgets import QTableWidget, QTableWidgetItem


class SensorTable(QTableWidget):
    def __init__(self) -> None:
        super().__init__(0, 6)
        self.setHorizontalHeaderLabels(
            ["Device ID", "Name", "Protocol", "Location", "Enabled", "Metrics"]
        )

    def populate(self, devices: list[dict]) -> None:
        self.setRowCount(len(devices))
        for row, device in enumerate(devices):
            metrics = ", ".join(metric["metric"] for metric in device.get("metrics", []))
            self.setItem(row, 0, QTableWidgetItem(device.get("id", "")))
            self.setItem(row, 1, QTableWidgetItem(device.get("name", "")))
            self.setItem(row, 2, QTableWidgetItem(device.get("protocol", "")))
            self.setItem(row, 3, QTableWidgetItem(device.get("location", "")))
            self.setItem(row, 4, QTableWidgetItem(str(device.get("enabled", False))))
            self.setItem(row, 5, QTableWidgetItem(metrics))

    def show_error(self, message: str) -> None:
        self.setRowCount(1)
        self.setItem(0, 0, QTableWidgetItem("error"))
        self.setItem(0, 1, QTableWidgetItem(message))
        for column in range(2, self.columnCount()):
            self.setItem(0, column, QTableWidgetItem("-"))
