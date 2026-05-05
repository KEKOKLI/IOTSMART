from __future__ import annotations

from PySide6.QtWidgets import (
    QDialog,
    QDialogButtonBox,
    QFormLayout,
    QLabel,
    QLineEdit,
    QMessageBox,
    QPushButton,
    QStackedWidget,
    QVBoxLayout,
    QWidget,
)


class LoginDialog(QDialog):
    def __init__(self, api_client, parent=None) -> None:
        super().__init__(parent)
        self.api_client = api_client
        self.user_info: dict | None = None

        self.setWindowTitle("Secure Access")
        self.setMinimumWidth(420)

        layout = QVBoxLayout(self)
        self.banner = QLabel("Authenticate to the IoT control core.")
        self.banner.setObjectName("AppTitle")
        self.hint = QLabel("")
        self.hint.setProperty("muted", True)

        self.stack = QStackedWidget()
        self.login_page = self._build_login_page()
        self.setup_page = self._build_setup_page()
        self.stack.addWidget(self.login_page)
        self.stack.addWidget(self.setup_page)

        layout.addWidget(self.banner)
        layout.addWidget(self.hint)
        layout.addWidget(self.stack)

        self.refresh_mode()

    def refresh_mode(self) -> None:
        status = self.api_client.auth_status()
        if status.get("setup_required"):
            self.hint.setText("No admin account exists yet. Create one to continue.")
            self.stack.setCurrentWidget(self.setup_page)
        else:
            self.hint.setText("Sign in to access device settings, charts, and logs.")
            self.stack.setCurrentWidget(self.login_page)

    def _build_login_page(self) -> QWidget:
        widget = QWidget()
        layout = QVBoxLayout(widget)
        form = QFormLayout()
        self.login_username = QLineEdit("admin")
        self.login_password = QLineEdit()
        self.login_password.setEchoMode(QLineEdit.Password)
        form.addRow("Username", self.login_username)
        form.addRow("Password", self.login_password)
        layout.addLayout(form)

        button_box = QDialogButtonBox()
        submit = QPushButton("Login")
        submit.setProperty("accent", True)
        submit.clicked.connect(self._do_login)
        button_box.addButton(submit, QDialogButtonBox.AcceptRole)
        layout.addWidget(button_box)
        return widget

    def _build_setup_page(self) -> QWidget:
        widget = QWidget()
        layout = QVBoxLayout(widget)
        form = QFormLayout()
        self.setup_username = QLineEdit("admin")
        self.setup_password = QLineEdit()
        self.setup_password.setEchoMode(QLineEdit.Password)
        self.setup_confirm = QLineEdit()
        self.setup_confirm.setEchoMode(QLineEdit.Password)
        form.addRow("Username", self.setup_username)
        form.addRow("Password", self.setup_password)
        form.addRow("Confirm", self.setup_confirm)
        layout.addLayout(form)

        button_box = QDialogButtonBox()
        submit = QPushButton("Create Admin")
        submit.setProperty("accent", True)
        submit.clicked.connect(self._do_bootstrap)
        button_box.addButton(submit, QDialogButtonBox.AcceptRole)
        layout.addWidget(button_box)
        return widget

    def _do_bootstrap(self) -> None:
        if self.setup_password.text() != self.setup_confirm.text():
            QMessageBox.warning(self, "Password mismatch", "The passwords do not match.")
            return
        try:
            self.api_client.bootstrap_admin(self.setup_username.text(), self.setup_password.text())
            self.api_client.login(self.setup_username.text(), self.setup_password.text())
            self.user_info = {"username": self.setup_username.text()}
            self.accept()
        except RuntimeError as exc:
            QMessageBox.warning(self, "Setup failed", str(exc))

    def _do_login(self) -> None:
        try:
            response = self.api_client.login(self.login_username.text(), self.login_password.text())
            self.user_info = response.get("user")
            self.accept()
        except RuntimeError as exc:
            QMessageBox.warning(self, "Login failed", str(exc))
