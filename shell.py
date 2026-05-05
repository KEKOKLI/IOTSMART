from __future__ import annotations

from pathlib import Path
import os
import socket
import subprocess
import time
from typing import Iterable
from urllib.error import URLError
from urllib.request import urlopen


class BackendManager:
    def __init__(self, workspace_root: Path) -> None:
        self.workspace_root = workspace_root
        self.process: subprocess.Popen[bytes] | None = None

    def ensure_running(self) -> bool:
        if self._healthcheck():
            return True

        executable = self._find_backend_executable()
        if executable is None:
            return False

        config_path = executable.parent / "configs" / "app.yaml"
        command = [str(executable)]
        if config_path.exists():
            command.extend(["-config", str(config_path)])

        self.process = subprocess.Popen(
            command,
            cwd=executable.parent,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        for _ in range(10):
            if self._healthcheck():
                return True
            time.sleep(1)
        return False

    def _healthcheck(self) -> bool:
        try:
            with urlopen("http://127.0.0.1:18080/api/v1/health", timeout=2):
                return True
        except socket.timeout:
            return False
        except URLError:
            return False

    def _find_backend_executable(self) -> Path | None:
        env_override = os.environ.get("IOT_BACKEND_PATH")
        if env_override:
            candidate = Path(env_override)
            if candidate.exists():
                return candidate

        candidates: Iterable[Path] = (
            self.workspace_root / "iot-backend.exe",
            self.workspace_root / "backend.exe",
            self.workspace_root / "bin" / "iot-backend.exe",
            self.workspace_root / "cmd" / "iot-backend" / "iot-backend.exe",
        )
        for candidate in candidates:
            if candidate.exists():
                return candidate
        return None
