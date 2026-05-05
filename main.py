from __future__ import annotations

from pathlib import Path
import sys

from PySide6.QtCore import Qt, QTimer
from PySide6.QtWidgets import (
    QApplication,
    QFrame,
    QHBoxLayout,
    QLabel,
    QListWidget,
    QMainWindow,
    QMessageBox,
    QPushButton,
    QScrollArea,
    QStackedWidget,
    QVBoxLayout,
    QWidget,
)

from app.api_client import APIClient
from app.shell import BackendManager
from app.theme import APP_STYLESHEET
from pages.dashboard_page import DashboardPage
from pages.database_page import DatabasePage
from pages.devices_page import DevicesPage
from pages.graph_builder_page import GraphBuilderPage
from pages.logs_page import LogsPage
from pages.protocol_settings_page import ProtocolSettingsPage
from pages.security_page import SecurityPage
from widgets.login_dialog import LoginDialog


class MainWindow(QMainWindow):
    def __init__(self) -> None:
        super().__init__()
        self.setWindowTitle("IoTsmart Control Nexus")
        self.resize(1500, 920)

        base_dir = Path(__file__).resolve().parent
        self.backend = BackendManager(base_dir.parent / "backend")
        self.api = APIClient()
        self.workspace_collapsed = False

        if not self.backend.ensure_running():
            QMessageBox.warning(
                self,
                "Backend unavailable",
                "The backend service could not be started. The UI will stay open, but protected pages may remain empty.",
            )

        if not self._ensure_authenticated():
            self.close()
            return

        root = QWidget(objectName="RootShell")
        root_layout = QVBoxLayout(root)
        root_layout.setContentsMargins(0, 0, 0, 0)
        root_layout.setSpacing(0)

        root_layout.addWidget(self._build_top_bar())
        body = QWidget()
        body_layout = QHBoxLayout(body)
        body_layout.setContentsMargins(0, 0, 0, 0)
        body_layout.setSpacing(0)
        self.sidebar = self._build_sidebar()
        body_layout.addWidget(self.sidebar)
        body_layout.addWidget(self._build_pages(), 1)
        root_layout.addWidget(body, 1)

        self.setCentralWidget(root)

        self.nav.currentRowChanged.connect(self.stack.setCurrentIndex)
        self.nav.currentRowChanged.connect(lambda _: self.refresh_current_page())
        self.nav.setCurrentRow(0)

        self.refresh_timer = QTimer(self)
        self.refresh_timer.timeout.connect(self.refresh_current_page)
        self.refresh_timer.timeout.connect(self._refresh_topbar)
        self.refresh_timer.start(12000)

        self._refresh_topbar()
        self.refresh_current_page()

    def _build_top_bar(self) -> QWidget:
        widget = QWidget(objectName="TopBar")
        layout = QHBoxLayout(widget)
        layout.setContentsMargins(28, 18, 28, 18)
        layout.setSpacing(14)

        title_block = QVBoxLayout()
        title = QLabel("IoTsmart Control Center")
        title.setObjectName("AppTitle")
        subtitle = QLabel("Keep sensors running, spot problems quickly, and export telemetry when you need it.")
        subtitle.setObjectName("AppSubtitle")
        title_block.addWidget(title)
        title_block.addWidget(subtitle)

        self.backend_status = QLabel("System: checking")
        self.backend_status.setProperty("pill", True)
        self.session_status = QLabel("Session: local")
        self.session_status.setProperty("pill", True)

        layout.addLayout(title_block)
        self.workspace_toggle = QPushButton("Hide Workspace")
        self.workspace_toggle.setProperty("secondary", True)
        self.workspace_toggle.setToolTip("Collapse or show the left Workspace navigation.")
        self.workspace_toggle.clicked.connect(self.toggle_workspace)
        layout.addWidget(self.workspace_toggle)
        refresh_button = QPushButton("Refresh Now")
        refresh_button.setProperty("secondary", True)
        refresh_button.setToolTip("Refresh the active page and top status immediately.")
        refresh_button.clicked.connect(self.refresh_all)
        layout.addWidget(refresh_button)
        layout.addStretch(1)
        layout.addWidget(self.backend_status)
        layout.addWidget(self.session_status)
        return widget

    def _build_sidebar(self) -> QWidget:
        widget = QWidget(objectName="Sidebar")
        widget.setFixedWidth(230)
        layout = QVBoxLayout(widget)
        layout.setContentsMargins(16, 18, 16, 18)
        layout.setSpacing(12)

        label = QLabel("Workspace")
        label.setObjectName("SectionTitle")

        self.nav = QListWidget()
        self.nav.addItems(
            [
                "Dashboard",
                "Devices",
                "Visuals",
                "Database",
                "Protocols",
                "Security",
                "Logs",
            ]
        )

        layout.addWidget(label)
        layout.addWidget(self.nav, 1)
        return widget

    def _build_pages(self) -> QWidget:
        container = QWidget(objectName="ContentShell")
        layout = QVBoxLayout(container)
        layout.setContentsMargins(24, 22, 24, 22)

        self.stack = QStackedWidget()
        self.pages = {
            "Dashboard": DashboardPage(self.api),
            "Devices": DevicesPage(self.api),
            "Visuals": GraphBuilderPage(self.api),
            "Database": DatabasePage(self.api),
            "Protocols": ProtocolSettingsPage(self.api),
            "Security": SecurityPage(self.api),
            "Logs": LogsPage(self.api),
        }
        if hasattr(self.pages["Dashboard"], "set_navigation_handler"):
            self.pages["Dashboard"].set_navigation_handler(self.navigate_to_page)
        for page in self.pages.values():
            self.stack.addWidget(self._wrap_page(page))

        layout.addWidget(self.stack, 1)
        return container

    def _wrap_page(self, page: QWidget) -> QScrollArea:
        scroller = QScrollArea(objectName="PageScroller")
        scroller.setWidgetResizable(True)
        scroller.setFrameShape(QFrame.NoFrame)
        scroller.setHorizontalScrollBarPolicy(Qt.ScrollBarAlwaysOff)
        scroller.setVerticalScrollBarPolicy(Qt.ScrollBarAlwaysOff)
        scroller.setWidget(page)
        return scroller

    def _ensure_authenticated(self) -> bool:
        try:
            status = self.api.auth_status()
        except RuntimeError:
            return True

        if not status.get("require_login"):
            return True

        dialog = LoginDialog(self.api, self)
        if dialog.exec() != LoginDialog.Accepted:
            return False
        return True

    def _refresh_topbar(self) -> None:
        try:
            health = self.api.health()
            status = str(health.get("status") or "unknown")
            self.backend_status.setText(
                f"System {status} | MQTT {health.get('mqtt', 'unknown')} | {health.get('online_devices', 0)} online"
            )
            self._set_pill_state(self.backend_status, "ok" if status == "ok" else "warn")
        except RuntimeError as exc:
            self.backend_status.setText(f"System offline: {exc}")
            self._set_pill_state(self.backend_status, "bad")

        session_text = "locked" if not self.api.token else "authenticated"
        self.session_status.setText(f"Session: {session_text}")
        self._set_pill_state(self.session_status, "ok" if self.api.token else "warn")

    def refresh_current_page(self) -> None:
        page = self._current_page()
        if hasattr(page, "refresh"):
            try:
                page.refresh()
            except Exception as exc:  # pragma: no cover - desktop runtime safety net
                QMessageBox.warning(self, "Refresh error", str(exc))

    def refresh_all(self) -> None:
        self._refresh_topbar()
        self.refresh_current_page()

    def navigate_to_page(self, page_name: str) -> None:
        items = self.nav.findItems(page_name, Qt.MatchExactly)
        if items:
            self.nav.setCurrentItem(items[0])

    def toggle_workspace(self) -> None:
        self.workspace_collapsed = not self.workspace_collapsed
        self.sidebar.setVisible(not self.workspace_collapsed)
        if self.workspace_collapsed:
            self.workspace_toggle.setText("Show Workspace")
            self.workspace_toggle.setToolTip("Show the left Workspace navigation.")
        else:
            self.workspace_toggle.setText("Hide Workspace")
            self.workspace_toggle.setToolTip("Collapse the left Workspace navigation.")

    @staticmethod
    def _set_pill_state(label: QLabel, state: str) -> None:
        label.setProperty("pillState", state)
        label.style().unpolish(label)
        label.style().polish(label)

    def _current_page(self) -> QWidget:
        current = self.stack.currentWidget()
        if isinstance(current, QScrollArea) and current.widget() is not None:
            return current.widget()
        return current


if __name__ == "__main__":
    app = QApplication(sys.argv)
    app.setStyleSheet(APP_STYLESHEET)
    window = MainWindow()
    window.show()
    sys.exit(app.exec())
