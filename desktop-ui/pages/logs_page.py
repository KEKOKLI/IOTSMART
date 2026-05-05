from __future__ import annotations

from PySide6.QtWidgets import QLabel, QPlainTextEdit, QVBoxLayout, QWidget


class LogsPage(QWidget):
    def __init__(self, api_client) -> None:
        super().__init__()
        self.api_client = api_client
        layout = QVBoxLayout(self)
        title = QLabel("Runtime Logs")
        title.setProperty("accent", True)
        self.viewer = QPlainTextEdit()
        self.viewer.setReadOnly(True)
        layout.addWidget(title)
        layout.addWidget(self.viewer, 1)

    def refresh(self) -> None:
        try:
            payload = self.api_client.logs(limit=200)
            lines = payload.get("lines", [])
            self.viewer.setPlainText("\n".join(lines))
        except RuntimeError as exc:
            self.viewer.setPlainText(str(exc))
