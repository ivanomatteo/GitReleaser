# Git Releaser

CLI Go per versionare e rilasciare in modo indipendente i microservizi di un monorepo. I tag Git nel formato `<service>/v<semver>` sono l'unica fonte delle versioni: il progetto non usa file `VERSION`.

Supporta anche i repo standard con l'opzione --root, l'utilità in questo caso è ovviamente inferiore e non necessita di file di configurazione.

## Che problema risolve?

In un monorepo con più microservizi, il repository Git è condiviso ma le release spesso sono indipendenti.

Un servizio può essere alla `2.4.1`, un altro alla `1.8.3`, e una modifica a un modulo `common` può rendere necessario rilasciare solo alcuni servizi.

Il problema è quindi capire in modo semplice:

* qual è l’ultima release di ogni servizio;
* quali modifiche sono avvenute da quella release;
* quali servizi risultano realmente `affected`;
* quale sarà la prossima versione o il prossimo tag.

Molti tool gestiscono il versioning mantenendo la versione anche nei sorgenti, ad esempio in `package.json`, `pom.xml` o file `VERSION`.

Questo progetto evita volutamente un secondo stato da sincronizzare e usa una sola source of truth:

```text id="pazl0w"
Git tag = release effettiva
```

Ogni servizio utilizza tag namespacizzati:

```text id="3vctve"
api/v2.4.1
worker/v1.8.3
scraper/v0.6.0
```

Per determinare se un servizio è cambiato, il tool confronta `HEAD` con il suo ultimo tag:

```text id="qb4rxb"
api/v2.4.1..HEAD
worker/v1.8.3..HEAD
```

Ogni servizio ha quindi una baseline indipendente, anche se tutti condividono la stessa history Git.

Le dipendenze condivise vengono dichiarate esplicitamente:

```yaml id="7kzr6a"
services:
  api:
    paths:
      - services/api
    dependencies:
      - common/auth
      - common/logging
```

Se cambia una dipendenza, il servizio viene considerato `affected` rispetto alla propria ultima release.

### Perché non usare un tool più completo?

Esistono strumenti come Nx Release, Changesets, Lerna o release-please che offrono funzionalità molto più estese: changelog, Conventional Commits, publishing, dependency graph automatici e integrazione con specifici ecosistemi.

Questo progetto è **volutamente minimale**.

Non cerca di gestire build, deployment, package registry, changelog o workflow Git.

Fa essenzialmente quattro cose:

```text id="5bvgm2"
trova l'ultima release
determina se il servizio è cambiato
calcola la prossima versione
crea il relativo tag
```

Lavora solo con:

```text id="puqgq7"
Git
path
dipendenze dichiarate
SemVer
```

Per questo rimane indipendente dal linguaggio, dal build system e dalla piattaforma CI/CD.

L’idea di fondo è semplice:

```text id="u0ec5a"
monorepo != mono-version
repository != release unit
```

Il repository rappresenta lo stato complessivo del codice.

I tag rappresentano invece le release delle singole unità deployabili.


## Requisiti e build

Sono richiesti Go 1.22 o successivo e Git disponibile nel `PATH`.

I binari precompilati per Linux, macOS e Windows sono disponibili nella [release più recente su GitHub](https://github.com/ivanomatteo/GitReleaser/releases/latest).

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

Per inizializzare in una sola operazione tutti i servizi configurati che non hanno ancora un tag valido, usa `--new`:

```sh
releaser release --new
# crea <service>/v0.1.0 per ogni servizio mai rilasciato

releaser release --new --version 1.0.0
# usa 1.0.0 come versione iniziale
```

I servizi che hanno già almeno un tag valido secondo lo schema `<service>/v<semver>` vengono ignorati. Se `--version` non è specificato, la versione iniziale predefinita è `0.1.0`. `--new` non accetta un nome di servizio o un bump e non può essere combinato con `--affected`, `--all`, `--root` o `--force`; supporta invece `--dry-run` e `--push`.

`--version` e un bump non possono essere usati insieme. Il parsing è strict: valori come `1.2`, `v1.2.3` e `01.2.3` non sono accettati.

Il tag rimane locale salvo richiesta esplicita:

```sh
releaser release api patch --push
```

Con `--push`, il comando pubblica soltanto il nuovo tag sul remote configurato. Non vengono eseguiti fetch, pull, build o deploy impliciti.

### Release in blocco

Per applicare lo stesso bump a tutti e soli i servizi affected:

```sh
releaser release --affected patch
```

Per applicarlo a tutti i servizi configurati, inclusi quelli non affected, occorre confermare esplicitamente con `--force`:

```sh
releaser release --all --force major
```

Il comando valida l'intero batch prima di creare tag. Ogni servizio deve avere una release precedente, perché la versione esplicita non è supportata in modalità bulk. `--dry-run` e `--push` sono disponibili anche per le release in blocco; con `--push` ogni tag creato viene pubblicato sul remote configurato.

### Repository standard (root)

Per rilasciare un repository standard, non organizzato come monorepo, usa `--root`. In questa modalità `releaser.yml` non è letto:

```sh
releaser release --root patch
releaser release --root --version 1.0.0
```

Il tag predefinito non ha prefix, per esempio `v0.1.5`. Se esiste già una release, il prefix viene dedotto dai tag precedenti; se vengono trovati prefix eterogenei, `--prefix` diventa obbligatorio. Un valore esplicitamente vuoto (`--prefix=''`) seleziona tag senza prefix:

```sh
releaser release --root patch --prefix=''
releaser release --root minor --prefix='release-'
```

Se non ci sono commit con modifiche dopo il tag precedente, la release richiede `--force`. `--dry-run` e `--push` sono supportati; il push usa `origin`. `--root` è mutuamente esclusivo con `--affected` e `--all`.

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
releaser release --affected <patch|minor|major> [--dry-run] [--push]
releaser release --all --force <patch|minor|major> [--dry-run] [--push]
releaser release --root <patch|minor|major> [--prefix=<prefix>] [--dry-run] [--push] [--force]
releaser release --root --version <semver> [--prefix=<prefix>] [--dry-run] [--push]
```

Usa `releaser --help` o `releaser <comando> --help` per l'help integrato.
