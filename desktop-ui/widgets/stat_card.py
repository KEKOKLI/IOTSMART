from PySide6.QtWidgets import QFrame, QLabel, QVBoxLayout


class StatCard(QFrame):
    def __init__(self, title: str, value: str, caption: str = "") -> None:
        super().__init__()
        self.setProperty("card", True)
        layout = QVBoxLayout(self)
        layout.setContentsMargins(18, 16, 18, 16)
        layout.setSpacing(8)
        self.setMinimumHeight(128)
        self.title = QLabel(title)
        self.title.setObjectName("HeroLabel")
        self.value = QLabel(value)
        self.value.setObjectName("HeroValue")
        self.value.setWordWrap(True)
        self.caption = QLabel(caption)
        self.caption.setProperty("muted", True)
        self.caption.setWordWrap(True)
        layout.addWidget(self.title)
        layout.addWidget(self.value)
        layout.addWidget(self.caption)
        layout.addStretch(1)

    def update_values(self, value: str, caption: str = "") -> None:
        self.value.setText(value)
        self.caption.setText(caption)
