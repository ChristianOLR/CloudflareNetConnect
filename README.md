# CloudflareNetConnect

Servicio desacoplado y demonio en Go para conectividad multi-cuenta simultánea a Cloudflare con soporte multiplataforma (*Windows, macOS, Linux*), privacidad absoluta (*Zero-Device Enrollment*) y bypass de limitaciones de WARP.

---

## Características Principales

- **Multi-Cuenta Concurrente:** Conexión en paralelo a organizaciones independientes en simultáneo.
- **Control del Servicio del Sistema:** Gestión nativa del ciclo de vida (`install`, `uninstall`, `start`, `stop`, `restart`, `status`) compatible con Windows SCM (`services.msc`), macOS `launchd` y Linux `systemd`.
- **System Tray e Identidad Visual:** Icono de Cloudflare incrustado (`assets/cloudflare.svg`), Master Toggle (ON/OFF) con reposo a 0% CPU.
- **Compilación Estática:** Binario único autosuficiente sin dependencias externas (`CGO_ENABLED=0`).

---

## Estructura del Componente

| Directorio / Archivo | Descripción |
| :--- | :--- |
| `cmd/daemon/main.go` | Punto de entrada CLI y modo demonio (<80 líneas). |
| `internal/config/` | Carga y guardado seguro de perfiles multi-cuenta (`netconnect.json`). |
| `internal/service/` | Abstracción del servicio del SO mediante `kardianos/service`. |
| `internal/tunnel/` | Orquestador concurrente de workers y enrutador de sockets TCP. |
| `internal/tray/` | Gestor del System Tray y Master Toggle. |
| `assets/` | Icono oficial de Cloudflare incrustado. |
| `Scripts/Build-CrossPlatform.ps1` | Pipeline de compilación cruzada para Windows, macOS y Linux. |

---

## Comandos Operativos de Servicio

```powershell
# Instalar como servicio del sistema operativo
.\CloudflareNetConnect.exe -service install

# Iniciar o detener el servicio
.\CloudflareNetConnect.exe -service start
.\CloudflareNetConnect.exe -service stop

# Consultar estado
.\CloudflareNetConnect.exe -service status

# Desinstalar el servicio
.\CloudflareNetConnect.exe -service uninstall
```

---

## Compilación Multiplataforma

Ejecutar el script automatizado:
```powershell
pwsh -NoProfile -NonInteractive -File Scripts/Build-CrossPlatform.ps1
```
Genera los binarios en `bin/`:
- `CloudflareNetConnect.exe` (Windows x64)
- `CloudflareNetConnect-linux` (Linux x64)
- `CloudflareNetConnect-mac-arm64` (macOS Apple Silicon)
