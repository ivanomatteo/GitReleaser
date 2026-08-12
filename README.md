# releaser

CLI Go per versionare e rilasciare in modo indipendente i microservizi di un monorepo. I tag Git nel formato `<service>/v<semver>` sono l'unica fonte delle versioni: il progetto non usa file `VERSION`.

## Requisiti e build

Sono richiesti Go 1.22 o successivo e Git disponibile nel `PATH`.

```sh
go build -o releaser ./cmd/releaser
```

Tutti i comandi operano sul repository corrente e leggono `releaser.yml`. Le opzioni globali permettono di cambiare entrambi:

```text
-c, --config string   file di configurazione (default "releaser.yml")
    --repo string     directory del repository Git (default ".")
```

## Configurazione

```yaml
remote: origin

ignore:
  - docs/**
  - "**/*.md"

services:
  api:
    paths:
      - services/api
    dependencies:
      - common/auth
      - common/logging
    ignore:
      - services/api/docs/**

  worker:
    paths:
      - services/worker
    dependencies:
      - common/logging
```

`paths` identifica il codice del servizio, `dependencies` aggiunge directory condivise che possono renderlo affected e `ignore` esclude glob specifici. L'`ignore` alla radice vale per tutti i servizi. `remote` è usato solo da `release --push` e, se omesso, vale `origin`.

Verifica la configurazione con:

```sh
releaser config check
```

## Comandi di versione

Questi comandi scrivono su stdout esclusivamente il valore richiesto, quindi possono essere usati direttamente negli script. Richiedono che il servizio abbia già almeno un tag valido.

```sh
releaser version-number api
# 2.4.1

releaser version-tag api
# api/v2.4.1

releaser next-version-number api patch
# 2.4.2

releaser next-version-tag api minor
# api/v2.5.0
```

I bump ammessi sono `patch`, `minor` e `major`. I comandi `next-version-*` calcolano soltanto il valore e non creano tag.

Esempio CI:

```sh
VERSION=$(releaser version-number api)
IMAGE_TAG=$(releaser next-version-number api patch)
```

## Stato e modifiche

`status` confronta, per ogni servizio, il suo ultimo tag con `HEAD` e considera sia `paths` sia `dependencies`:

```sh
releaser status
releaser status api
releaser status api --verbose
```

L'output compatto riporta servizio, ultima versione e stato affected. La modalità verbose include anche il tag e i file modificati. Un servizio privo di release è considerato affected e mostra `none` come versione.

Per ottenere solo i nomi dei servizi affected, uno per riga:

```sh
releaser affected
```

Per ottenere solo i file che rendono affected un servizio:

```sh
releaser changes api
```

I comandi read-only confrontano commit Git e ignorano le modifiche non committate nella working tree.

## Piano di release

```sh
releaser plan
releaser plan --format json
```

Il formato tabellare mostra versione, stato affected e numero di file modificati. Il formato JSON è adatto alle pipeline:

```json
{
  "services": [
    {
      "name": "api",
      "lastVersion": "2.4.1",
      "lastTag": "api/v2.4.1",
      "affected": true,
      "changedFiles": 2
    }
  ]
}
```

Per un servizio mai rilasciato, `lastVersion` e `lastTag` valgono `null`.

## Creazione di una release

Per incrementare l'ultima versione e creare un tag annotato su `HEAD`:

```sh
releaser release api patch
releaser release api minor
releaser release api major
```

Il tag risultante ha forma `api/v2.4.2` e messaggio `Release api v2.4.2`. La working tree deve essere pulita.

Il servizio deve essere affected, cioè deve avere modifiche rilevanti successive alla sua ultima release. In caso contrario il comando termina con un errore. Per creare intenzionalmente un tag anche senza modifiche del servizio, usa `--force`:

```sh
releaser release api patch --force
```

Per vedere il risultato senza creare il tag:

```sh
releaser release api minor --dry-run
```

Per il bootstrap di un servizio senza tag, o per impostare una versione precisa, usa una SemVer valida senza prefisso `v`:

```sh
releaser release new-service --version 1.0.0
```

`--version` e un bump non possono essere usati insieme. Il parsing è strict: valori come `1.2`, `v1.2.3` e `01.2.3` non sono accettati.

Il tag rimane locale salvo richiesta esplicita:

```sh
releaser release api patch --push
```

Con `--push`, il comando pubblica soltanto il nuovo tag sul remote configurato. Non vengono eseguiti fetch, pull, build o deploy impliciti.

## Exit code ed errori

Gli errori sono scritti su stderr con prefisso `ERROR:`. Gli exit code sono:

| Codice | Significato |
| ---: | --- |
| 0 | successo |
| 1 | input non valido, servizio/versione non disponibile o stato non valido |
| 2 | configurazione non leggibile o non valida |
| 3 | repository, history o operazione Git non disponibile |

In una shallow clone occorre rendere disponibili tag e history necessari prima di eseguire il tool; `releaser` non accede automaticamente alla rete.

## Elenco rapido

```text
releaser config check
releaser status [service] [--verbose]
releaser affected
releaser changes <service>
releaser version-number <service>
releaser version-tag <service>
releaser next-version-number <service> <patch|minor|major>
releaser next-version-tag <service> <patch|minor|major>
releaser plan [--format table|json]
releaser release <service> <patch|minor|major> [--dry-run] [--push] [--force]
releaser release <service> --version <semver> [--dry-run] [--push] [--force]
```

Usa `releaser --help` o `releaser <comando> --help` per l'help integrato.
