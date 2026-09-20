<p align="center">
  <img src="https://img.shields.io/badge/renfecli-CLI%20de%20Renfe-blue?style=for-the-badge" alt="renfecli">
</p>

<h1 align="center">renfecli</h1>

<p align="center">
  <strong>CLI no oficial de Renfe, pensada también para agentes — horarios y tarifas reales de tren desde la terminal</strong>
</p>

<p align="center">
  <a href="https://github.com/seifreed/renfecli/releases"><img src="https://img.shields.io/github/v/tag/seifreed/renfecli?style=flat-square&logo=go&logoColor=white&label=version" alt="Versión"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/go-1.26.6%2B-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Versión de Go"></a>
  <a href="https://github.com/seifreed/renfecli/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="Licencia"></a>
  <a href="https://github.com/seifreed/renfecli/actions/workflows/quality.yml"><img src="https://img.shields.io/github/actions/workflow/status/seifreed/renfecli/quality.yml?style=flat-square&logo=github&label=quality" alt="Puerta de calidad"></a>
  <a href="https://github.com/seifreed/renfecli/actions/workflows/security.yml"><img src="https://img.shields.io/github/actions/workflow/status/seifreed/renfecli/security.yml?style=flat-square&logo=github&label=security" alt="Puerta de seguridad"></a>
  <a href="https://github.com/seifreed/renfecli/security/code-scanning"><img src="https://img.shields.io/badge/code%20scanning-SARIF%20enabled-brightgreen?style=flat-square" alt="SARIF"></a>
</p>

<p align="center">
  <a href="https://github.com/seifreed/renfecli/stargazers"><img src="https://img.shields.io/github/stars/seifreed/renfecli?style=flat-square" alt="Estrellas en GitHub"></a>
  <a href="https://github.com/seifreed/renfecli/issues"><img src="https://img.shields.io/github/issues/seifreed/renfecli?style=flat-square" alt="Issues en GitHub"></a>
  <a href="https://buymeacoffee.com/seifreed"><img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-support-yellow?style=flat-square&logo=buy-me-a-coffee&logoColor=white" alt="Buy Me a Coffee"></a>
</p>

---

## Resumen

**renfecli** busca y tarifica viajes en tren desde la línea de comandos. Habla los mismos endpoints
HTTP que usa `venta.renfe.com` — sin navegador headless y sin raspar HTML ya renderizado — y todos
los comandos emiten JSON o TOON para uso programático.

Es de **solo lectura**: busca, tarifica y consulta tu propia cuenta. Nunca reserva, ni paga, ni
modifica nada.

```console
$ renfe search madrid barcelona --date tomorrow --cheapest --limit 5
MADRID-PUERTA DE ATOCHA-ALMUDENA GRANDES → BARCELONA-SANTS   21/09/2026   1 adult

AVE       06:27 → 09:50  3h 23m     tren 3063               from 49.80 €  [cheapest]
AVE       07:27 → 11:11  3h 44m     tren 3073               from 49.80 €
AVE       09:27 → 13:04  3h 37m     tren 3093               from 59.60 €
AVE       08:57 → 12:06  3h 09m     tren 3091               from 70.35 €
AVE       06:16 → 09:24  3h 08m     tren 3301               from 124.50 €

cheapest nearby: 2026-09-22 from 37 €  (renfe calendar for the full strip)
```

> La salida de la CLI está en inglés; los nombres de estación y los avisos son los que manda Renfe.

### Características principales

| Característica | Descripción |
|---------|-------------|
| **Tarifas en vivo** | Todos los tipos de tarifa por tren, no solo el precio de portada |
| **Calendario de precios** | La tira de precio mínimo por día de Renfe, gratis con cada búsqueda |
| **Ida y vuelta** | Ambos sentidos en una sola petición, comprobando si la vuelta en el día es posible |
| **Paradas intermedias** | Estaciones de paso y prestaciones a bordo, por tramo en los enlaces |
| **Catálogo de estaciones** | 1367 estaciones, con búsqueda difusa y caché; incluye Francia y Portugal |
| **Tu cuenta** | Tarjeta +Renfe y próximos viajes, a partir de una sesión que ya tienes |
| **Mantener la sesión** | Mantiene viva una sesión inactiva en lugar de volver a extraerla cada media hora |
| **Salida para agentes** | `--json` y `--toon` en todos los comandos; datos a stdout, logs a stderr |
| **Huella TLS de Chrome** | JA3 vía uTLS para que la capa antibots no plantee retos |
| **Multiplataforma** | Probado en Linux, macOS y Windows; cada tag publica binarios para los tres |

### Salidas disponibles

```text
Viajes              Tabla legible, JSON, TOON
Calendario          Tabla legible, JSON, TOON
Itinerarios         Tabla legible, JSON, TOON
Códigos de salida   0 correcto · 1 error de ejecución · 2 error de uso
Flujos              datos → stdout, diagnósticos → stderr
```

---

## Instalación

### Desde una release

Cada tag `v*` publica binarios para Linux, macOS (Intel y Apple Silicon) y Windows, y cada archivo
lleva al lado su fichero `SHA256SUMS`. Descarga el de tu plataforma desde
[Releases](https://github.com/seifreed/renfecli/releases), verifícalo y pon `renfe` en tu `PATH`:

```bash
tar xzf renfe_v1.0.0_darwin_arm64.tar.gz
shasum -c renfe_v1.0.0_darwin_SHA256SUMS
```

### Con Go

```bash
go install github.com/seifreed/renfecli/cmd/renfe@latest
```

### Desde el código fuente

```bash
git clone https://github.com/seifreed/renfecli.git
cd renfecli
make build          # ./renfe
```

### Extras opcionales

`make tools` instala las herramientas fijadas de las puertas (golangci-lint, govulncheck,
cyclonedx-gomod) que usan `make quality` y `make security`.

---

## Inicio rápido

```bash
# Viajes y tarifas para una fecha
renfe search madrid barcelona --date tomorrow

# Qué día sale más barato
renfe calendar madrid sevilla --date +3

# Buscar el código de una estación
renfe stations valencia

# ¿Dónde para este tren?
renfe stops madrid cadiz --date +2 --train 2074
```

---

## Uso

### Interfaz de línea de comandos

```bash
# Ida y vuelta, salida por la tarde y regreso por la noche
renfe search madrid sevilla --date friday --return sunday --after 15:00 --return-after 18:00

# Todos los tipos de tarifa, ocultando los trenes agotados
renfe search barcelona zaragoza --date +5 --fares --available

# Para scripts y agentes
renfe search madrid vigo --date +14 --toon
```

### Comandos disponibles

| Comando | Descripción |
|--------|-------------|
| `renfe search` | Viajes con tarifas en vivo para una fecha, con filtros y ordenación |
| `renfe calendar` | Precio mínimo por día alrededor de una fecha (solo corredores principales) |
| `renfe stops` | Paradas intermedias de un tren y prestaciones a bordo |
| `renfe stations` | Nombres y códigos de estación |
| `renfe login` | Extrae una sesión iniciada desde tu navegador |
| `renfe whoami` | Confirma la sesión y muestra la tarjeta +Renfe |
| `renfe trips` | Próximos viajes de la cuenta |
| `renfe session` | Estado de la sesión, y `session keep` para mantenerla viva |

### Opciones de búsqueda

| Opción | Descripción |
|--------|-------------|
| `--date <d>` | `YYYY-MM-DD`, `DD/MM/YYYY`, `today`, `tomorrow`, un día de la semana, o `+N` días |
| `--return <d>` | Ida y vuelta: ambos sentidos, tarificados con reglas de ida y vuelta |
| `--adults N` `--children N` `--infants N` | Viajeros. **Todos los precios son por pasajero** |
| `--after HH:MM` `--before HH:MM` | Acotan la salida de ida (`--return-*` para la vuelta) |
| `--direct` | Excluye los viajes con enlace |
| `--cheapest` `--available` `--fares` `--limit N` | Ordenar por precio · ocultar lo no comprable · todas las tarifas · limitar la lista |
| `--pet` `--bike` `--wheelchair` | Viajar con mascota, con bicicleta o necesitando plaza H |
| `--json` `--toon` | Salida estructurada (datos a stdout, diagnósticos a stderr) |

---

## Puertas de calidad y seguridad

El equivalente en Go de una cadena `black` / `ruff` / `mypy` / `bandit` / `pip-audit`, más las
comprobaciones que Go necesita y Python no.

```bash
make check       # la puerta del día a día: fmt, vet, tests, build, lint
make quality     # + go.mod ordenado, detector de carreras, mínimo de cobertura
make security    # SAST, vulnerabilidades de dependencias, integridad de módulos
make gate        # ambas
```

| Aspecto | Python | En este proyecto |
|---------|--------|------------------|
| Formato | `black --check` | `gofmt -l` |
| Linting | `ruff` | `golangci-lint` (staticcheck, revive, gocritic, errcheck, ineffassign, unused) |
| Tipos | `mypy` | `go vet` + el compilador |
| SAST | `bandit` | `gosec`, ejecutado como linter de golangci-lint |
| CVEs de dependencias | `pip-audit` | `govulncheck` (con análisis de alcanzabilidad) |
| Higiene del lockfile | `pip-compile --check` | diff de `go mod tidy` |
| Integridad | hashes en el lockfile | `go mod verify` |
| Concurrencia | — | `go test -race` |
| SBOM | `cyclonedx-py` | `cyclonedx-gomod` |

CI ejecuta ambas puertas en cada push y pull request, con la suite de tests y el detector de
carreras en **ubuntu-latest, windows-latest y macos-latest**; la de seguridad corre además
semanalmente, sube SARIF a GitHub Code Scanning y ejecuta CodeQL. Publicar un tag `v*` crea el
borrador de una release, compila en las tres plataformas (Linux y macOS para amd64 y arm64, Windows
para amd64), adjunta los archivos con sus checksums y solo la publica cuando todas han salido bien.

> **Nota sobre `gosec`:** ninguna versión publicada de `gosec` independiente analiza la biblioteca
> estándar de Go 1.27 (`internal error: package "bufio" without types`). Se ejecuta como linter de
> `golangci-lint`, así que la cobertura SAST no se resiente; `make gosec` se mantiene como objetivo
> de mejor esfuerzo, que informa de la situación en lugar de hacer fallar la puerta.

---

## Skill para Claude Code

`.claude/skills/renfe-trains/` incluye una skill para que Claude Code use la CLI correctamente:
cuándo resolver primero una estación, por qué `calendar` sustituye a un bucle de búsquedas diarias,
qué significan los tipos de tarifa, y la línea roja de que esta herramienta nunca reserva ni paga.
`references/cli-reference.md` documenta cada comando y cada forma de `--json`. La skill está en
inglés, que es el idioma en el que opera el agente.

---

## Cómo funciona

El backend de venta de Renfe es una aplicación Java que expone sus beans de servicio por
[DWR](https://github.com/directwebremoting/dwr): el navegador envía por POST un cuerpo de texto
orientado a líneas que nombra un bean y un método, y el servidor responde con un programa
JavaScript que llama de vuelta a la página. `renfe` habla ese protocolo directamente — sin navegador
headless y sin motor JS — en tres pasos:

1. `GET /vol/inicio.do` para obtener una sesión,
2. `POST /vol/buscarTren.do` con el formulario de búsqueda, que guarda la ruta en el servidor,
3. `POST …/trainEnlacesManager.getTrainsList.dwr`, cuya respuesta trae los viajes, todos los tipos
   de tarifa y el calendario de precios.

DWR hace además de protección CSRF propia: el `scriptSessionId` de una llamada debe llevar la cookie
`DWRSESSIONID` como prefijo. Las peticiones presentan la huella TLS (JA3) de Chrome mediante
[uTLS](https://github.com/refraction-networking/utls).

---

## Cosas que conviene saber

- **Los precios son por pasajero**, nunca el total del grupo, y son **tarifas base**: los descuentos
  de Tarjeta Dorada, Carné Joven y Familia Numerosa se aplican más adelante en el flujo de compra de
  Renfe. Multiplica por los viajeros **con asiento**: un menor de 4 años indicado con `--infants`
  viaja en brazos y no entra en esa cifra.
- **`stops` muestra el viaje que pediste, no el recorrido entero del tren.** Muchos servicios siguen
  más allá de tu destino — el 07:27 Madrid–Barcelona continúa hasta Figueres — y las paradas
  terminan en tu estación, con tu hora de llegada.
- **La ventana de venta de Renfe es corta** — medida en 83 días en Madrid–Barcelona — y avanza a
  tramos con el horario, no día a día.
- **Algunos enlaces cambian de estación.** Barcelona→Santander llega a Madrid-Atocha y sale de
  Chamartín; `search` y `stops` imprimen el aviso de Renfe.
- **Un tren al que solo le quedan plazas H sigue dando precio.** Se marca como tal y queda fuera de
  `--available` salvo que hayas pasado `--wheelchair`.
- **Cercanías (líneas C) no se vende por este flujo**, y otros operadores (Ouigo, Iryo) le son
  invisibles: una comparación de precios aquí es solo de Renfe.
- **Los viajes transfronterizos a Francia y Portugal funcionan** (`AVE INT`, `TRENCELT`). Las
  estaciones extranjeras aparecen con su nombre local, con los nombres españoles más comunes
  aliasados. No hay Lisboa.
- **Una pestaña del navegador que parece con sesión iniciada a menudo no lo está**: su fichero de
  cookies conserva las de una sesión ya muerta, así que `login` verifica contra el servidor antes de
  guardar nada, y prueba todos los navegadores en lugar de quedarse con el primero que lo aparenta.
- **Las peticiones imposibles se rechazan, no se buscan.** Una fecha ya pasada, un `--return`
  anterior a la ida, un número de viajeros negativo: Renfe responde a las dos primeras con algo que
  parece un resultado vacío normal, así que se detectan antes de gastar una petición.

---

## Configuración

`~/.renfe/config.toml` (el directorio se cambia con `RENFE_CONFIG_DIR`):

```toml
[defaults]
origin = "Madrid"        # se usa cuando se omite <origen>
destination = "Sevilla"  # ídem para <destino>
adults = 2
```

También puedes indicar una sesión a mano, en lugar de usar `renfe login`:

```toml
[auth]
cookie = "JSESSIONID=…; SSOInfo=…"   # deja este fichero en modo 600
```

`renfe` avisa si los permisos de cualquiera de los dos ficheros permiten leerlo a otros usuarios de
la máquina.

### Entorno

| Variable | Efecto |
|----------|--------|
| `RENFE_CONFIG_DIR` | Cambia `~/.renfe` |
| `RENFE_BASE_URL` | Apunta el cliente a un proxy, un mock o un servidor de replay |
| `RENFE_ALLOW_SESSION_ON_CUSTOM_HOST` | Ponla a `1` para que ese host reciba tu sesión |

`RENFE_BASE_URL` redirige las peticiones **pero no la credencial**: un host que no sea
`venta.renfe.com` ni esta máquina no recibe tu sesión guardada, y los comandos de cuenta fallan en
cerrado en lugar de seguir en anónimo. La mitad anónima sigue funcionando allí. Habilita un host
solo si de verdad debe tener tu sesión.

El catálogo de estaciones se cachea 7 días; `renfe stations --update` lo refresca.

---

## Requisitos

- Go 1.26.6+ — las versiones anteriores de la serie 1.26 arrastran fallos de
  `crypto/x509`, `crypto/tls` y `net/http` en la biblioteca estándar, y este
  cliente depende de la verificación de certificados
- Las dependencias están en [go.mod](go.mod)

---

## Contribuir

Las contribuciones son bienvenidas.

1. Haz un fork del repositorio
2. Crea tu rama (`git checkout -b feature/amazing-feature`)
3. Pasa las puertas (`make gate`)
4. Haz commit de tus cambios (`git commit -m 'Add amazing feature'`)
5. Sube la rama (`git push origin feature/amazing-feature`)
6. Abre un Pull Request

---

## Apoyar el proyecto

Si este proyecto te resulta útil, puedes apoyar su desarrollo:

<a href="https://buymeacoffee.com/seifreed" target="_blank">
  <img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" height="50">
</a>

---

## Licencia

Este proyecto se publica bajo licencia MIT. Consulta [LICENSE](LICENSE).

Sin afiliación con Renfe ni respaldo por su parte.

**Atribución**
- Autor: **Marc Rivero López** | [@seifreed](https://github.com/seifreed)
- Repositorio: [github.com/seifreed/renfecli](https://github.com/seifreed/renfecli)

---

<p align="center">
  <sub>Hecho para planificar trenes de verdad y para automatización con agentes</sub>
</p>
