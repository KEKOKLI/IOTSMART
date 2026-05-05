from PySide6.QtWidgets import QLabel


class StatusBadge(QLabel):
    def __init__(self, text: str, color: str = "#1f6c5c") -> None:
        super().__init__(text)
        self.setStyleSheet(
            f"background:{color}; color:white; border-radius:9px; padding:4px 10px; font-weight:600;"
        )
