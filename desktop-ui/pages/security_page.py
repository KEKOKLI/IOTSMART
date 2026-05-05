from __future__ import annotations

from PySide6.QtWidgets import (
    QFormLayout,
    QFrame,
    QHBoxLayout,
    QLabel,
    QLineEdit,
    QMessageBox,
    QPushButton,
    QSpinBox,
    QTableWidget,
    QTableWidgetItem,
    QVBoxLayout,
    QWidget,
)


class SecurityPage(QWidget):
    def __init__(self, api_client) -> None:
        super().__init__()
        self.api_client = api_client
        self.tokens_cache: list[dict] = []

        layout = QVBoxLayout(self)
        layout.addWidget(self._build_status_card())
        layout.addWidget(self._build_password_card())
        layout.addWidget(self._build_token_card(), 1)

    def _build_status_card(self) -> QWidget:
        panel = QFrame()
        panel.setProperty("card", True)
        layout = QVBoxLayout(panel)

        title = QLabel("Account Access")
        title.setProperty("accent", True)
        self.status_label = QLabel("Checking authentication state...")
        self.status_label.setProperty("muted", True)

        form = QFormLayout()
        self.username_input = QLineEdit("admin")
        self.password_input = QLineEdit()
        self.password_input.setEchoMode(QLineEdit.Password)
        form.addRow("Username", self.username_input)
        form.addRow("Password", self.password_input)

        button_row = QHBoxLayout()
        login_button = QPushButton("Login / Refresh Session")
        login_button.setProperty("accent", True)
        login_button.clicked.connect(self._login)
        logout_button = QPushButton("Logout")
        logout_button.setProperty("danger", True)
        logout_button.clicked.connect(self._logout)
        button_row.addWidget(login_button)
        button_row.addWidget(logout_button)
        button_row.addStretch(1)

        layout.addWidget(title)
        layout.addWidget(self.status_label)
        layout.addLayout(form)
        layout.addLayout(button_row)
        return panel

    def _build_password_card(self) -> QWidget:
        panel = QFrame()
        panel.setProperty("glass", True)
        layout = QVBoxLayout(panel)

        title = QLabel("Change Password")
        title.setProperty("accent", True)
        hint = QLabel("Use at least 8 characters. Existing login token stays active until logout.")
        hint.setProperty("muted", True)

        form = QFormLayout()
        self.current_password_input = QLineEdit()
        self.current_password_input.setEchoMode(QLineEdit.Password)
        self.new_password_input = QLineEdit()
        self.new_password_input.setEchoMode(QLineEdit.Password)
        self.confirm_password_input = QLineEdit()
        self.confirm_password_input.setEchoMode(QLineEdit.Password)
        form.addRow("Current password", self.current_password_input)
        form.addRow("New password", self.new_password_input)
        form.addRow("Confirm password", self.confirm_password_input)

        change_button = QPushButton("Update Password")
        change_button.setProperty("accent", True)
        change_button.clicked.connect(self._change_password)

        layout.addWidget(title)
        layout.addWidget(hint)
        layout.addLayout(form)
        layout.addWidget(change_button)
        return panel

    def _build_token_card(self) -> QWidget:
        panel = QFrame()
        panel.setProperty("card", True)
        layout = QVBoxLayout(panel)

        title = QLabel("API Tokens")
        title.setProperty("accent", True)

        token_form = QHBoxLayout()
        self.token_name_input = QLineEdit("desktop-automation")
        self.token_days_input = QSpinBox()
        self.token_days_input.setRange(1, 3650)
        self.token_days_input.setValue(365)
        create_button = QPushButton("Generate Token")
        create_button.setProperty("accent", True)
        create_button.clicked.connect(self._create_token)
        delete_button = QPushButton("Delete Selected Token")
        delete_button.setProperty("danger", True)
        delete_button.clicked.connect(self._delete_token)
        token_form.addWidget(self.token_name_input, 1)
        token_form.addWidget(self.token_days_input)
        token_form.addWidget(create_button)
        token_form.addWidget(delete_button)

        self.token_value_label = QLabel("New tokens will appear here once generated.")
        self.token_value_label.setProperty("muted", True)

        self.table = QTableWidget(0, 5)
        self.table.setHorizontalHeaderLabels(["ID", "Name", "Kind", "Expires", "Last used"])

        layout.addWidget(title)
        layout.addLayout(token_form)
        layout.addWidget(self.token_value_label)
        layout.addWidget(self.table, 1)
        return panel

    def refresh(self) -> None:
        try:
            status = self.api_client.auth_status()
        except RuntimeError as exc:
            self.status_label.setText(str(exc))
            return

        self.status_label.setText(
            f"Require login: {status.get('require_login')} | Setup required: {status.get('setup_required')} | Token loaded: {bool(self.api_client.token)}"
        )
        if self.api_client.token:
            self._refresh_tokens()
        else:
            self.tokens_cache = []
            self.table.setRowCount(0)

    def _login(self) -> None:
        try:
            self.api_client.login(self.username_input.text(), self.password_input.text())
            self.status_label.setText("Authenticated. Tokens and protected endpoints are now available.")
            self._refresh_tokens()
        except RuntimeError as exc:
            QMessageBox.warning(self, "Login failed", str(exc))

    def _logout(self) -> None:
        try:
            self.api_client.logout()
        except RuntimeError as exc:
            QMessageBox.warning(self, "Logout warning", f"Local token was cleared, but backend logout failed: {exc}")
        finally:
            self.tokens_cache = []
            self.table.setRowCount(0)
            self.status_label.setText("Logged out. Local session token has been revoked.")

    def _change_password(self) -> None:
        if not self.api_client.token:
            QMessageBox.information(self, "Login required", "Log in before changing the password.")
            return
        new_password = self.new_password_input.text()
        if new_password != self.confirm_password_input.text():
            QMessageBox.warning(self, "Password mismatch", "New password and confirmation do not match.")
            return
        try:
            self.api_client.change_password(self.current_password_input.text(), new_password)
            self.current_password_input.clear()
            self.new_password_input.clear()
            self.confirm_password_input.clear()
            self.status_label.setText("Password updated.")
        except RuntimeError as exc:
            QMessageBox.warning(self, "Password update failed", str(exc))

    def _refresh_tokens(self) -> None:
        try:
            self.tokens_cache = self.api_client.list_tokens()
        except RuntimeError as exc:
            self.status_label.setText(str(exc))
            return

        self.table.setRowCount(len(self.tokens_cache))
        for row, token in enumerate(self.tokens_cache):
            values = [
                token.get("id", ""),
                token.get("name", ""),
                token.get("kind", ""),
                str(token.get("expires_at", "")),
                str(token.get("last_used_at", "")),
            ]
            for column, value in enumerate(values):
                self.table.setItem(row, column, QTableWidgetItem(value))

    def _create_token(self) -> None:
        if not self.api_client.token:
            QMessageBox.information(self, "Login required", "Log in before creating API tokens.")
            return
        try:
            response = self.api_client.create_token(
                self.token_name_input.text().strip() or "desktop-automation",
                self.token_days_input.value(),
            )
            self.token_value_label.setText(f"Plain token: {response.get('access_token', '')}")
            self._refresh_tokens()
        except RuntimeError as exc:
            QMessageBox.warning(self, "Token error", str(exc))

    def _delete_token(self) -> None:
        row = self.table.currentRow()
        if row < 0 or row >= len(self.tokens_cache):
            QMessageBox.information(self, "No token selected", "Select a token to delete.")
            return
        try:
            self.api_client.delete_token(self.tokens_cache[row].get("id", ""))
            self._refresh_tokens()
        except RuntimeError as exc:
            QMessageBox.warning(self, "Delete failed", str(exc))
