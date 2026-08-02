

# devreport

[![Go](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org/) [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**DevReport** es una herramienta de línea de comandos que genera informes automatizados de actividad individual a partir de tareas de ClickUp. Recopila tareas, calcula estadísticas (totales, completadas, por estado/origen/tipo) y genera informes HTML/PDF atractivos con resúmenes y tablas detalladas.

## Características

- **Informes con un solo comando**: Obtén datos de GitHub/ClickUp → Genera HTML/PDF + exportación JSON
- **Estadísticas inteligentes**: Calcula automáticamente totales, tasas de finalización y desgloses por estado/origen/tipo
- **Multiplataforma**: Binarios precompilados para macOS, Linux, Windows (AMD64/ARM64)
- **Reformulación con IA opcional**: Se integra con [Ollama](https://ollama.com/) para reescribir tareas  

## Requisitos previos

- **Cuenta de GitHub**: Token de acceso personal para la API (opcional, para repositorios privados)
- **Cuenta de ClickUp**: Acceso a tareas mediante API
- **Terminal**: sh/Zsh/PowerShell
- **Opcional**: Navegador web para visualizar los informes HTML
- **Opcional**: [Ollama](https://ollama.com/) ejecutándose localmente (puerto 11434) para la reformulación con IA  

## Instalación de Ollama y Gemma3 en Linux

1. Instalar Ollama:

   ```sh
   curl -fsSL https://ollama.com/install.sh | sh
   ```

2. Iniciar Ollama:

   ```sh
   ollama serve
   ```

3. Descargar el modelo Gemma3:

   ```sh
   ollama pull gemma3
   ```

4. Verificar la instalación:

   ```sh
   curl http://localhost:11434/api/tags
   ```

---

## Instalación

### Opción 1: Descargar binario precompilado (Recomendado)

1. Ve a la [página de Releases](https://github.com/Afrawles/devreport/releases)
2. Descarga el archivo para tu plataforma:
   - macOS Intel: `devreport_darwin_amd64.tar.gz`
   - macOS Apple Silicon: `devreport_darwin_arm64.tar.gz`
   - Linux AMD64: `devreport_linux_amd64.tar.gz`
   - Linux ARM64: `devreport_linux_arm64.tar.gz`
   - Windows: `devreport_windows_amd64.zip`

3. Extrae e instala (macOS/Linux):

   ```sh
   tar -xzf devreport_darwin_amd64.tar.gz
   chmod +x devreport
   sudo mv devreport /usr/local/bin/
   ```

4. Verificar la instalación:

   ```sh
   devreport --help
   ```

### Opción 2: Compilar desde el código fuente

```sh
git clone https://github.com/Afrawles/devreport.git
cd devreport
go build -o devreport ./cmd/devreport
./devreport --help
```

---

## Obtener tu token de API de ClickUp

1. Inicia sesión en [ClickUp](https://app.clickup.com)
2. Haz clic en tu foto de perfil → **Configuración**
3. Ve a **Apps** → **API Token**
4. Haz clic en **Generate** y copia tu token
5. Úsalo con la bandera `--clickup-token`

---

## Encontrar tus IDs de listas de ClickUp

1. En la barra lateral de ClickUp, pasa el cursor sobre una Lista  
2. Haz clic en los **puntos suspensivos (...)** → **Copiar enlace**  
3. Ejemplo:

   ```sh
   https://app.clickup.com/12345678/v/li/987654321
   ```

4. El número después de `/li/` es tu **ID de Lista** (`987654321`)  
5. Usa comas para separar varias listas:

   ```sh
   987654321,123456789
   ```

---

## Encontrar los IDs de asignados de ClickUp

```sh
curl -H "Authorization: YOUR_API_TOKEN" \
  "https://api.clickup.com/api/v2/team"
```

---

## Obtener tu token de acceso personal de GitHub

1. Inicia sesión en [GitHub](https://github.com)
2. Haz clic en tu foto de perfil → **Configuración**
3. Ve a **Developer settings** → **Personal access tokens** → **Tokens (classic)**
4. Haz clic en **Generate new token (classic)**
5. Selecciona los ámbitos (scopes): `repo` (para repos privados), `read:org` (para repos de organización)
6. Copia el token y úsalo con la bandera `--github-token`

---

## Encontrar tus organizaciones de GitHub

Para obtener actividades de repositorios de organizaciones, especifica los nombres de las organizaciones (por ejemplo, "microsoft,google").

Nota: DevReport obtiene datos de los repositorios donde tienes actividades recientes en el rango de fechas especificado para limitar las llamadas a la API.

---

## Uso

Cuando se trabaja con múltiples listas de ClickUp, DevReport asigna el texto según el orden de las listas.

- Usa comas (`,`) para separar **listas diferentes**
- Usa barras verticales (`|`) dentro de cada grupo de lista para separar **oraciones que pertenecen a esa lista**

### Ejemplo de estructura

```sh
--clickup-listid "11111111,33333333" \
--challenges "Delayed client feedback|Unclear UI specifications|Integration issues with payment service, Server maintenance downtime|Third-party API instability|Deployment delays" \
--support-required "Product team review|QA support for test coverage|DevOps for CI/CD automation, Management alignment|Database admin support|Load testing assistance" \
--support-from "Product Management|QA Department|DevOps Team, IT Infrastructure|Backend Team|Project Management Office" \
--follow-up "Conduct sprint retrospective|Optimize frontend performance|Write integration tests, Refactor legacy modules|Enhance documentation|Evaluate monitoring tools"
```

Explicación:

- Todo lo que esté antes de la primera coma (`,`) pertenece a la **Lista 11111111**  
- Todo lo que esté después de la coma pertenece a la **Lista 33333333**  
- Dentro de cada grupo, `|` separa las oraciones para esa lista  

---

### Comando básico (ClickUp)

```sh
./devreport \
  --user "Uzumaki.Gon" \
  --start "2025-10-01" \
  --end "2025-10-31" \
  --author "Killua Uzumaki" \
  --period "Month of October" \
  --year 2025 \
  --category "Improvements, New Features, and Bug Fixes" \
  --clickup-token "your_clickup_token_here" \
  --clickup-assignees 1234536,1728383 \
  --clickup-listid "11111111,33333333" \
  --challenges "Delayed client feedback|Unclear UI specifications|Integration issues with payment service, Server maintenance downtime|Third-party API instability|Deployment delays" \
  --support-required "Product team review|QA support for test coverage|DevOps for CI/CD automation, Management alignment|Database admin support|Load testing assistance" \
  --support-from "Product Management|QA Department|DevOps Team, IT Infrastructure|Backend Team|Project Management Office" \
  --follow-up "Conduct sprint retrospective|Optimize frontend performance|Write integration tests, Refactor legacy modules|Enhance documentation|Evaluate monitoring tools"
```

### Comando para GitHub

```sh
./devreport \
  --user "gon" \
  --start "2025-10-01" \
  --end "2025-10-31" \
  --author "Gon Freecss" \
  --period "Month of October" \
  --year 2025 \
  --github-token "your_github_token_here" \
  --github-orgs "hunterxhunter,chimera-ant" \
  --github-include-reviewed-prs \
  --github-include-assigned-issues
```

### Comando combinado (GitHub + ClickUp)

```sh
./devreport \
  --user "gon" \
  --start "2025-10-01" \
  --end "2025-10-31" \
  --author "Gon Freecss" \
  --period "Month of October" \
  --year 2025 \
  --clickup-token "clickup_token" \
  --clickup-assignees 12345 \
  --clickup-listid "11111" \
  --github-token "github_token" \
  --github-orgs "hunterxhunter" \
  --github-include-reviewed-prs
```

Después de la ejecución, abre el informe generado:

```sh
reports/report_Uzumaki.Gon_20251030.html
```

Este archivo (exportación del informe) contiene el resumen de tareas, secciones categorizadas y contenido reformulado por IA (si Ollama está disponible).

### Para equipos de Operaciones y Negocios

```bash
devreport summary --period <period> --clickup-token "<token>" --clickup-folderid <folder-id>

```

## Períodos admitidos

| Período | Descripción | Ejemplo |
|--------|-------------|---------|
| `today` | Día actual | Tareas creadas hoy |
| `yesterday` | Día anterior | Tareas creadas ayer |
| `this-week` o `thisweek` | Semana actual (Lun-Dom) | Tareas del lunes hasta ahora |
| `last-week` o `lastweek` | Semana anterior (Lun-Dom) | Tareas del lunes pasado hasta el domingo |
| `this-month` o `thismonth` | Mes actual | Tareas del día 1 hasta ahora |
| `last-month` o `lastmonth` | Mes anterior | Tareas del día 1 al último día del mes anterior |
| `all-time` o `alltime` | Todo el tiempo | Todas las tareas creadas hasta la fecha |

## Ejemplos

### Ejemplos de ClickUp

```bash
# Weekly report
devreport summary --period this-week --clickup-token "pk_xxx" --clickup-folderid 123456

# Monthly report
devreport summary --period this-month --clickup-token "pk_xxx" --clickup-folderid 123456

# Filter by specific assignees
devreport summary --period this-week --clickup-token "pk_xxx" --clickup-folderid 123456 --clickup-assignees "user1,user2"

# Use environment variables
export CLICKUP_API_KEY="pk_xxx"
export CLICKUP_FOLDERID="123456"
devreport summary --period this-week
```

### Ejemplos de GitHub

```bash
# GitHub activities report
devreport --user "gon" --start "2025-10-01" --end "2025-10-31" --github-token "ghp_xxx" --github-orgs "hunterxhunter"

# Include reviewed PRs
devreport --user "gon" --start "2025-10-01" --end "2025-10-31" --github-token "ghp_xxx" --github-orgs "hunterxhunter" --github-include-reviewed-prs

# Use environment variables
export GITHUB_TOKEN="ghp_xxx"
export GITHUB_ORGS="hunterxhunter,chimera-ant"
devreport --user "gon" --period this-month
```
