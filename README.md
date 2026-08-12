# releaser

CLI Go per versionare e rilasciare in modo indipendente i microservizi di un monorepo. Git tag nel formato `<service>/v<semver>` sono l'unica fonte delle versioni.

## Build

```sh
go build -o releaser ./cmd/releaser
```

Il file predefinito è `releaser.yml`; un percorso diverso può essere passato con `--config`. Usa `releaser --help` per l'elenco completo dei comandi.

Esempio minimo:

```yaml
services:
  api:
    paths:
      - services/api
    dependencies:
      - common/logging
```

```sh
releaser config check
releaser status
releaser next-version-tag api patch
releaser release api patch --dry-run
```
