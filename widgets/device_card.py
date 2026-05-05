from PySide6.QtWidgets import QFrame, QLabel, QVBoxLayout

from widgets.status_badge import StatusBadge


class DeviceCard(QFrame):
    def __init__(self, device: dict) -> None:
        super().__init__()
        self.setProperty("card", True)
        layout = QVBoxLayout(self)
        layout.addWidget(QLabel(device.get("name", device.get("id", "Unknown device"))))
        layout.addWidget(StatusBadge("Online" if device.get("enabled") else "Disabled"))
        layout.addWidget(QLabel(f"Protocol: {device.get('protocol', '-') }"))
        layout.addWidget(QLabel(f"Location: {device.get('location', '-') }"))
