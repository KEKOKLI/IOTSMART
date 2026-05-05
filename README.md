# IoTSMART
All vibe code
Before dealing with VTC FYP project
Windows IoT desktop scaffold with a local Go backend and a PySide6 control-room UI.

## What is in this version

- Local Go backend with SQLite auto-create, WAL mode, simulated telemetry, REST API, and WebSocket feed.
- Real MQTT subscriber connector using topic wildcard subscription.
- Auth layer with admin bootstrap, login, bearer tokens, and token management endpoints.
- Device CRUD with metric definitions.
- Graph presets plus CSV export.
- Refreshed PySide6 UI with a darker, more futuristic dashboard and multi-chart builder.

## Backend features

- `GET /api/v1/health`
- `GET /api/v1/auth/status`
- `POST /api/v1/auth/bootstrap`
- `POST /api/v1/auth/login`
- `GET/POST /api/v1/auth/tokens`
- `DELETE /api/v1/auth/tokens/{id}`
- `GET/POST /api/v1/devices`
- `GET/PUT/DELETE /api/v1/devices/{id}`
- `POST /api/v1/ingest`
- `GET /api/v1/telemetry/latest`
- `GET /api/v1/telemetry/query`
- `GET /api/v1/telemetry/export`
- `GET/POST /api/v1/graphs`
- `PUT/DELETE /api/v1/graphs/{id}`
- `GET /api/v1/protocols/status`
- `POST /api/v1/protocols/mqtt/test`
- `POST /api/v1/protocols/mqtt/publish`
- `POST /api/v1/protocols/modbus/test`
- `POST /api/v1/protocols/modbus/read`
- `POST /api/v1/protocols/modbus/rtu/test`
- `POST /api/v1/protocols/modbus/rtu/read`
- `POST /api/v1/protocols/opcua/test`
- `POST /api/v1/protocols/opcua/browse`
- `POST /api/v1/protocols/opcua/read`
- `POST /api/v1/protocols/bacnet/test`
- `POST /api/v1/protocols/bacnet/read-property`
- `POST /api/v1/protocols/ads/test`
- `POST /api/v1/protocols/ads/read-symbol`
- `POST /api/v1/protocols/odbc/test`
- `POST /api/v1/protocols/odbc/query`
- `GET /api/v1/logs`
- `GET /ws/live`

## Supported MQTT payloads

Single reading:

```json
{
  "device_id": "mqtt_temp_001",
  "name": "Lab Temperature",
  "metric": "temperature",
  "value": 24.5,
  "unit": "C",
  "timestamp": "2026-04-27T12:00:00Z"
}
```

Multiple readings:

```json
{
  "device_id": "mqtt_env_001",
  "name": "Environment Sensor",
  "readings": [
    { "metric": "temperature", "value": 24.5, "unit": "C" },
    { "metric": "humidity", "value": 61.2, "unit": "%" }
  ]
}
```

Metrics object:

```json
{
  "device_id": "mqtt_env_002",
  "metrics": {
    "temperature": 23.8,
    "humidity": 58.4
  },
  "units": {
    "temperature": "C",
    "humidity": "%"
  }
}
```

## Run backend

```powershell
cd E:\IoTsmart\backend
go mod tidy
go build -o .\bin\iot-backend.exe .\cmd\iot-backend
.\bin\iot-backend.exe -config .\configs\app.yaml
```

The backend will auto-create:

- `E:\IoTsmart\backend\data\iot_app.db`
- `E:\IoTsmart\backend\logs\app.log`

## Run UI

If your existing `.venv` is broken, recreate it with Python 3.12:

```powershell
cd E:\IoTsmart\desktop-ui
Remove-Item -Recurse -Force .venv
C:\Users\LSL\AppData\Local\Programs\Python\Python312\python.exe -m venv .venv
.\.venv\Scripts\python.exe -m pip install --upgrade pip
.\.venv\Scripts\python.exe -m pip install -r requirements.txt
```

Then start the UI:

```powershell
cd E:\IoTsmart\desktop-ui
$env:IOT_BACKEND_PATH = "E:\IoTsmart\backend\bin\iot-backend.exe"
.\.venv\Scripts\python.exe .\main.py
```

## First login

`require_login` is enabled in [app.yaml](/E:/IoTsmart/backend/configs/app.yaml).

On first launch:

1. The login dialog will switch to admin setup mode.
2. Create the first admin account.
3. The UI will immediately log in and unlock protected pages.

## Graphs

The graph builder currently supports:

- `line`
- `spline`
- `area`
- `bar`
- `pie`
- `donut`

## Notes

- MQTT is optional. Enable it in [app.yaml](/E:/IoTsmart/backend/configs/app.yaml) and point `broker` plus `topic_prefix` to your broker.
- Graph presets and auth tokens are stored locally in SQLite.
- The simulated fleet still starts by default so the dashboard has live data even before you connect real sensors.
- WiX Toolset can package this into a Windows installer. Use PyInstaller for `IoTApp.exe`, `go build` for `iot-backend.exe`, then WiX MSI or a Burn `.exe` bundle to install both files plus `configs/`, `drivers/`, `data/`, and `logs/`.
