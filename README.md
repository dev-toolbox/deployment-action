# deployment-action

GitHub Action in Go, die die service-spezifischen Inhalte aus dem lokalen `_deployment`-Verzeichnis eines Repositories in das organisationsweite Repository `_deployment` synchronisiert.

## Verhalten

- Läuft für Service-Repositories auf `main` und `develop`.
- Wenn der aktuelle Commit auf `main` einen Tag im Muster `v*.*.*` trägt, wird nach `prod/<service>/` synchronisiert.
- Die dabei verwendete Version ist der Tag ohne führendes `v`.
- Wenn `main` keinen passenden Tag hat oder der Branch `develop` ist, wird nach `dev/<service>/` synchronisiert.
- In `dev` wird der 6-stellige Kurz-Hash des aktuellen Commits verwendet.
- In Textdateien des lokalen `_deployment`-Verzeichnisses wird standardmäßig `{{VERSION}}` ersetzt.
- Existiert das lokale `_deployment`-Verzeichnis nicht, räumt die Action den zugehörigen Service-Eintrag im Ziel-Repository auf.

## Zielstruktur im `_deployment`-Repository

- `prod/<service>/...`
- `dev/<service>/...`
- `prod/kustomization.yaml`
- `dev/kustomization.yaml`

In `prod/kustomization.yaml` oder `dev/kustomization.yaml` wird jeweils genau ein Eintrag `./<service>` gepflegt.

## Inputs

| Name | Pflicht | Default | Beschreibung |
| --- | --- | --- | --- |
| `token` | ja | - | GitHub Token mit Schreibrechten auf das organisationsweite `_deployment`-Repository |
| `deployment_repository` | nein | `_deployment` | Ziel-Repository in derselben GitHub-Organisation |
| `deployment_branch` | nein | `main` | Ziel-Branch im `_deployment`-Repository |
| `placeholder` | nein | `{{VERSION}}` | Platzhalter, der in Textdateien ersetzt wird |
| `source_directory` | nein | `_deployment` | Quellverzeichnis im Service-Repository |

## Outputs

| Name | Beschreibung |
| --- | --- |
| `stage` | `prod` oder `dev` |
| `version` | Version ohne führendes `v` oder 6-stelliger Kurz-Hash |
| `tag` | Verwendeter Git-Tag auf `main`, sonst leer |

## Voraussetzungen

- Das Ziel-Repository `_deployment` muss in derselben GitHub-Organisation existieren.
- Das übergebene Token muss Schreibrechte auf das Ziel-Repository besitzen.
- Die auslösenden Workflows sollten nur auf `main` und `develop` laufen.

Wenn das Ziel-Repository nicht existiert, bricht die Action mit einem Fehler ab.

## Beispiel

```yaml
name: Deploy manifests

on:
  push:
    branches:
      - main
      - develop

jobs:
  sync-deployment:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Sync deployment assets
        id: sync
        uses: dev-toolbox/deployment-action@main
        with:
          token: ${{ secrets.DEPLOYMENT_REPO_TOKEN }}

      - name: Show resolved target
        run: |
          echo "stage=${{ steps.sync.outputs.stage }}"
          echo "version=${{ steps.sync.outputs.version }}"
          echo "tag=${{ steps.sync.outputs.tag }}"
```

## Aufräumverhalten

Wenn das lokale `_deployment`-Verzeichnis fehlt, passiert für die relevante Stage Folgendes:

- Das Zielverzeichnis `<stage>/<service>/` wird entfernt, sofern Dateien vorhanden sind.
- Der Eintrag `./<service>` wird aus `<stage>/kustomization.yaml` entfernt, sofern vorhanden.
- Fehlen Verzeichnis und Eintrag bereits, beendet sich die Action erfolgreich und idempotent.

## Entwicklung

```bash
go test ./...
docker build -t deployment-action .
```

## Container-Veröffentlichung

Das Container-Image wird ausschließlich bei Versionstags im Muster `v*.*.*` gebaut und gepusht.

Diese Tags werden über cocogitto auf `main` erzeugt.

- Registry: `DOCKER_REGISTRY`
- Benutzer: `DOCKER_USER`
- Passwort: `DOCKER_PASS`
- Image-Name: `<DOCKER_REGISTRY>/<ORG>/<repo-name>`

Veröffentlichte Tags:

- `latest`
- exakte SemVer-Version ohne führendes `v`
- `major.minor`
- `major`

## Release-Automatisierung mit cocogitto

Auf Pushes nach `main` führt die Release-Workflow-Datei cocogitto aus.

- `cog check --from-latest-tag` validiert Conventional Commits in der CI.
- `cog bump --auto` erzeugt auf `main` den Version-Bump und einen Tag mit Präfix `v`.
- Der erzeugte Tag startet anschließend den Container-Publish-Workflow.
- Der gleiche v-Tag startet zusätzlich einen GitHub-Release-Workflow, der die Release Notes über cocogitto erzeugt.

Die cocogitto-Konfiguration liegt in `cog.toml` und setzt insbesondere:

- `tag_prefix = "v"`
- `from_latest_tag = true`
- `branch_whitelist = ["main"]`

## Vollständiger Release-Fluss

Ein typischer Ablauf sieht so aus:

1. Änderungen werden mit Conventional Commits nach `main` gemergt.
2. Die CI prüft die Commits seit dem letzten Tag mit cocogitto.
3. Der Release-Workflow führt `cog bump --auto` aus.
4. Cocogitto erstellt einen Release-Commit und einen neuen Tag im Format `v<major>.<minor>.<patch>`.
5. Der neue v-Tag startet:
   - den Container-Publish-Workflow
   - den GitHub-Release-Workflow mit automatisch erzeugten Release Notes

Beispiele für Commit-Typen und ihre Wirkung:

- `fix: correct deployment path handling`
  - typischerweise Patch-Bump
- `feat: add cleanup for missing deployment directory`
  - typischerweise Minor-Bump
- `feat!: switch deployment writes to atomic git commit`
  - typischerweise Major-Bump
- `refactor: simplify kustomization update flow`
  - kein automatischer Version-Bump, sofern nicht als bump-relevant konfiguriert

Ein Breaking Change kann auch über den Footer signalisiert werden:

```text
feat: redesign deployment payload

BREAKING CHANGE: deployment placeholders now require explicit VERSION tokens
```

Beispielhafter Ablauf auf `main`:

```text
feat: add deployment sync action
fix: remove stale service entries from kustomization
feat!: replace per-file writes with atomic git tree commit
```

Aus diesen Commits erzeugt cocogitto den nächsten passenden v-Tag und die zugehörigen Release Notes.

## GitHub Release Notes

Bei jedem Tag im Format `v*.*.*` wird ein GitHub Release angelegt.

- Workflow: `.github/workflows/github-release.yml`
- Changelog-Erzeugung: `cog changelog --at <tag>`
- Veröffentlichung: GitHub Release mit dem Tag-Namen und dem von cocogitto erzeugten Changelog

## Consumer-Beispiele

Ein Service-Repository kann die Lösung auf zwei Arten verwenden:

### Empfohlenes Secret- und Variable-Schema im Service-Repo

Empfohlene Secrets:

- `DEPLOYMENT_REPO_TOKEN`: Token mit Schreibrechten auf das organisationsweite `_deployment`-Repository
- `DOCKER_REGISTRY`: Registry-Host für das Action-Image
- `DOCKER_USER`: Registry-Benutzer
- `DOCKER_PASS`: Registry-Passwort

Empfohlene Repository-Variablen:

- `DEPLOYMENT_ACTION_PIN_PROD`: exakter Action-Tag, z. B. `v1.2.3`
- `DEPLOYMENT_ACTION_PIN_DEV`: gleitender Action-Tag auf Minor-Linie, z. B. `v1.2`
- `DEPLOYMENT_ACTION_IMAGE_PIN_PROD`: exakter Image-Tag, z. B. `v1.2.3`
- `DEPLOYMENT_ACTION_IMAGE_PIN_DEV`: gleitender Image-Tag auf Minor-Linie, z. B. `v1.2`

### Empfohlener Version-Pinning-Standard

- Production (`main`): exakter Patch-Pin, z. B. `v1.2.3`
- Development (`develop`): Minor-Pin, z. B. `v1.2`

So werden Breaking Changes kontrolliert ausgerollt, während `develop` weiterhin automatisch Patch-Fixes erhält.

### 1) GitHub Action direkt per Release-Tag

```yaml
name: Sync deployment assets

on:
  push:
    branches:
      - main
      - develop

jobs:
  sync-prod:
    if: github.ref_name == 'main'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Run deployment action (prod pin)
        id: deploy
        uses: dev-toolbox/deployment-action@v1.2.3
        with:
          token: ${{ secrets.DEPLOYMENT_REPO_TOKEN }}

      - name: Show resolved target
        run: |
          echo "stage=${{ steps.deploy.outputs.stage }}"
          echo "version=${{ steps.deploy.outputs.version }}"
          echo "tag=${{ steps.deploy.outputs.tag }}"

  sync-dev:
    if: github.ref_name == 'develop'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Run deployment action (dev pin)
        id: deploy
        uses: dev-toolbox/deployment-action@v1.2
        with:
          token: ${{ secrets.DEPLOYMENT_REPO_TOKEN }}

      - name: Show resolved target
        run: |
          echo "stage=${{ steps.deploy.outputs.stage }}"
          echo "version=${{ steps.deploy.outputs.version }}"
          echo "tag=${{ steps.deploy.outputs.tag }}"
```

Wenn keine Repository-Variablen verwendet werden sollen:

- `main` auf exakten Tag pinnen, z. B. `dev-toolbox/deployment-action@v1.2.3`
- `develop` auf Minor pinnen, z. B. `dev-toolbox/deployment-action@v1.2`

### 2) Veröffentlichtes Container-Image als Job-Container

```yaml
name: Validate released container

on:
  workflow_dispatch:

jobs:
  run-container-prod:
    if: github.ref_name == 'main'
    runs-on: ubuntu-latest
    container:
      image: ${{ secrets.DOCKER_REGISTRY }}/dev-toolbox/deployment-action:v1.2.3
      credentials:
        username: ${{ secrets.DOCKER_USER }}
        password: ${{ secrets.DOCKER_PASS }}
    env:
      GITHUB_REPOSITORY: dev-toolbox/my-service
      GITHUB_REF_NAME: main
      GITHUB_SHA: 0123456789abcdef0123456789abcdef01234567
      GITHUB_WORKSPACE: .
      INPUT_TOKEN: ${{ secrets.DEPLOYMENT_REPO_TOKEN }}
      INPUT_DEPLOYMENT_REPOSITORY: _deployment
      INPUT_DEPLOYMENT_BRANCH: main
      INPUT_PLACEHOLDER: "{{VERSION}}"
      INPUT_SOURCE_DIRECTORY: _deployment
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Execute binary from image
        run: /usr/local/bin/deployment-action

  run-container-dev:
    if: github.ref_name == 'develop'
    runs-on: ubuntu-latest
    container:
      image: ${{ secrets.DOCKER_REGISTRY }}/dev-toolbox/deployment-action:v1.2
      credentials:
        username: ${{ secrets.DOCKER_USER }}
        password: ${{ secrets.DOCKER_PASS }}
    env:
      GITHUB_REPOSITORY: dev-toolbox/my-service
      GITHUB_REF_NAME: develop
      GITHUB_SHA: 0123456789abcdef0123456789abcdef01234567
      GITHUB_WORKSPACE: .
      INPUT_TOKEN: ${{ secrets.DEPLOYMENT_REPO_TOKEN }}
      INPUT_DEPLOYMENT_REPOSITORY: _deployment
      INPUT_DEPLOYMENT_BRANCH: main
      INPUT_PLACEHOLDER: "{{VERSION}}"
      INPUT_SOURCE_DIRECTORY: _deployment
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Execute binary from image
        run: /usr/local/bin/deployment-action
```

Hinweis: Für die reguläre Nutzung im Service-Repo ist Variante 1 die bevorzugte Option.
Variante 2 ist vor allem nützlich für Smoke-Tests gegen ein explizites Image-Tag.

Wenn keine Repository-Variablen verwendet werden sollen:

- `main`: `<DOCKER_REGISTRY>/dev-toolbox/deployment-action:v1.2.3`
- `develop`: `<DOCKER_REGISTRY>/dev-toolbox/deployment-action:v1.2`

Die Implementierung ist bewusst in kleine Pakete getrennt:

- `internal/versioning`: Branch-, Tag- und Versionsauflösung
- `internal/syncer`: Rekursives Einlesen und Dateidiff
- `internal/kustomize`: Pflege von `kustomization.yaml`
- `internal/githubapi`: GitHub API Zugriff auf das Ziel-Repository
- `internal/app`: Orchestrierung des Gesamtflusses