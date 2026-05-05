from __future__ import annotations

import json
import socket
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen


class APIClient:
    def __init__(self, base_url: str = "http://127.0.0.1:18080") -> None:
        self.base_url = base_url.rstrip("/")
        self.token: str | None = None

    def set_token(self, token: str | None) -> None:
        self.token = token

    def auth_status(self) -> dict[str, Any]:
        return self._request("GET", "/api/v1/auth/status")

    def bootstrap_admin(self, username: str, password: str) -> dict[str, Any]:
        return self._request(
            "POST",
            "/api/v1/auth/bootstrap",
            payload={"username": username, "password": password},
        )

    def login(self, username: str, password: str) -> dict[str, Any]:
        response = self._request(
            "POST",
            "/api/v1/auth/login",
            payload={"username": username, "password": password},
        )
        token = response.get("access_token")
        if token:
            self.set_token(token)
        return response

    def logout(self) -> None:
        try:
            self._request("POST", "/api/v1/auth/logout")
        finally:
            self.set_token(None)

    def change_password(self, current_password: str, new_password: str) -> dict[str, Any]:
        return self._request(
            "POST",
            "/api/v1/auth/change-password",
            payload={"current_password": current_password, "new_password": new_password},
        )

    def list_tokens(self) -> list[dict[str, Any]]:
        return self._request("GET", "/api/v1/auth/tokens")

    def create_token(self, name: str, days: int = 365) -> dict[str, Any]:
        return self._request(
            "POST",
            "/api/v1/auth/tokens",
            payload={"name": name, "days": days},
        )

    def delete_token(self, token_id: str) -> None:
        self._request("DELETE", f"/api/v1/auth/tokens/{token_id}")

    def health(self) -> dict[str, Any]:
        return self._request("GET", "/api/v1/health")

    def devices(self) -> list[dict[str, Any]]:
        return self._request("GET", "/api/v1/devices")

    def device(self, device_id: str) -> dict[str, Any]:
        return self._request("GET", f"/api/v1/devices/{device_id}")

    def create_device(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/devices", payload=payload)

    def update_device(self, device_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("PUT", f"/api/v1/devices/{device_id}", payload=payload)

    def delete_device(self, device_id: str) -> None:
        self._request("DELETE", f"/api/v1/devices/{device_id}")

    def latest_telemetry(self, limit: int = 50) -> list[dict[str, Any]]:
        return self._request("GET", "/api/v1/telemetry/latest", params={"limit": limit})

    def query_telemetry(self, **params: Any) -> list[dict[str, Any]]:
        return self._request("GET", "/api/v1/telemetry/query", params=params)

    def export_csv(self, destination: str | Path, **params: Any) -> Path:
        content = self._request(
            "GET",
            "/api/v1/telemetry/export",
            params=params,
            return_bytes=True,
        )
        path = Path(destination)
        path.write_bytes(content)
        return path

    def graph_presets(self) -> list[dict[str, Any]]:
        return self._request("GET", "/api/v1/graphs")

    def create_graph_preset(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/graphs", payload=payload)

    def update_graph_preset(self, preset_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("PUT", f"/api/v1/graphs/{preset_id}", payload=payload)

    def delete_graph_preset(self, preset_id: str) -> None:
        self._request("DELETE", f"/api/v1/graphs/{preset_id}")

    def protocol_status(self) -> dict[str, Any]:
        return self._request("GET", "/api/v1/protocols/status")

    def mqtt_test_connection(self) -> dict[str, Any]:
        return self._request("POST", "/api/v1/protocols/mqtt/test")

    def mqtt_publish_test(self, topic: str, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request(
            "POST",
            "/api/v1/protocols/mqtt/publish",
            payload={"topic": topic, "payload": payload},
        )

    def modbus_test_connection(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/protocols/modbus/test", payload=payload)

    def modbus_read_registers(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/protocols/modbus/read", payload=payload)

    def modbus_rtu_test_connection(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/protocols/modbus/rtu/test", payload=payload)

    def modbus_rtu_read_registers(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/protocols/modbus/rtu/read", payload=payload)

    def opcua_test_connection(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/protocols/opcua/test", payload=payload)

    def opcua_browse(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/protocols/opcua/browse", payload=payload)

    def opcua_read_node(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/protocols/opcua/read", payload=payload)

    def bacnet_test_connection(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/protocols/bacnet/test", payload=payload)

    def bacnet_read_property(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/protocols/bacnet/read-property", payload=payload)

    def ads_test_connection(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/protocols/ads/test", payload=payload)

    def ads_read_symbol(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/protocols/ads/read-symbol", payload=payload)

    def odbc_test_connection(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/protocols/odbc/test", payload=payload)

    def odbc_query(self, payload: dict[str, Any]) -> dict[str, Any]:
        return self._request("POST", "/api/v1/protocols/odbc/query", payload=payload)

    def logs(self, limit: int = 200) -> dict[str, Any]:
        return self._request("GET", "/api/v1/logs", params={"limit": limit})

    def _request(
        self,
        method: str,
        path: str,
        payload: dict[str, Any] | None = None,
        params: dict[str, Any] | None = None,
        return_bytes: bool = False,
    ) -> Any:
        query = f"?{urlencode(params)}" if params else ""
        url = f"{self.base_url}{path}{query}"
        body = None
        headers = {"Content-Type": "application/json"}
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        if payload is not None:
            body = json.dumps(payload).encode("utf-8")

        request = Request(url, data=body, headers=headers, method=method)
        try:
            with urlopen(request, timeout=4) as response:
                content = response.read()
                if return_bytes:
                    return content
                if not content:
                    return None
                return json.loads(content.decode("utf-8"))
        except HTTPError as exc:
            message = exc.read().decode("utf-8")
            raise RuntimeError(f"HTTP {exc.code}: {message}") from exc
        except socket.timeout as exc:
            raise RuntimeError("backend request timed out") from exc
        except URLError as exc:
            raise RuntimeError(f"backend unavailable: {exc.reason}") from exc
