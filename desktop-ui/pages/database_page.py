from __future__ import annotations

from PySide6.QtWidgets import QLabel, QVBoxLayout, QWidget


class DatabasePage(QWidget):
    def __init__(self, api_client) -> None:
        super().__init__()
        self.api_client = api_client
        layout = QVBoxLayout(self)
        self.summary = QLabel("SQLite runs locally with automatic table creation and WAL journaling.")
        self.summary.setProperty("accent", True)
        self.details = QLabel("Use this page as the future home for retention, backup, and maintenance controls.")
        self.details.setProperty("muted", True)
        layout.addWidget(self.summary)
        layout.addWidget(self.details)
        layout.addStretch(1)

    def refresh(self) -> None:
        try:
            health = self.api_client.health()
        except RuntimeError as exc:
            self.summary.setText(str(exc))
            return
        self.summary.setText(
            f"Database: {health.get('db', 'unknown')} | Devices: {health.get('total_devices', 0)} | Last ingest: {health.get('last_ingest_at', 'n/a')}"
        )
