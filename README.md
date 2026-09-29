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

Sono richiesti Go 1.27 o successivo e Git disponibile nel `PATH`.

I binari precompilati per Linux, macOS e Windows sono disponibili nella [release più recente su GitHub](https://github.com/ivanomatteo/GitReleaser/releases/latest).

```sh
CGO_ENABLED=0 go build -o releaser ./cmd/releaser
```

Tutti i comandi operano sul repository corrente e leggono `releaser.yml`. Le opzioni globali permettono di cambiare entrambi:

```text
-c, --config string   file di configurazione (default "releaser.yml")
    --repo string     directory del repository Git (default ".")
```

Il `releaser.yml` predefinito viene cercato nella directory indicata da `--repo`, quindi `releaser --repo ../monorepo status` funziona da qualsiasi directory. Un `--config` relativo passato esplicitamente è invece risolto rispetto alla directory corrente.

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
    vars:
      image: registry.example.com/project/api
      deployment: api-production

  worker:
    paths:
      - services/worker
    dependencies:
      - common/logging
```

`paths` identifica il codice del servizio, `dependencies` aggiunge directory condivise che possono renderlo affected, `ignore` esclude glob specifici e `vars` contiene valori stringa chiave-valore specifici del servizio. L'`ignore` alla radice vale per tutti i servizi. `remote` è usato solo da `release --push` e, se omesso, vale `origin`.

`paths` è obbligatorio, gli altri campi sono facoltativi. Le chiavi sconosciute (per esempio un refuso come `dependecies`) sono un errore di configurazione. I nomi dei servizi devono essere validi come componente di un tag Git: niente `/`, spazi, `~ ^ : ? * [ \`, `..` o `@{`, e non possono iniziare con `.` o `-` né terminare con `.` o `.lock`.

I glob di `ignore` supportano `**` (qualsiasi numero di directory), `*` e `?` (all'interno di un singolo segmento di path); gli altri caratteri sono letterali. Per un file rinominato vengono valutati sia il path precedente sia quello nuovo: il servizio è affected se almeno uno dei due appartiene ai suoi `paths`/`dependencies` e non è ignorato, e `changes` riporta solo i path rilevanti.

Verifica la configurazione con:

```sh
releaser config check
```

Le variabili sono recuperabili senza accedere a Git e il comando stampa esclusivamente il valore, così da poterlo usare negli script:

```sh
releaser get-var api image
# registry.example.com/project/api

IMAGE=$(releaser get-var api image)
```

Il comando fallisce se il servizio o la chiave non esistono. I valori di `vars` devono essere stringhe; numeri e booleani vanno quindi racchiusi tra virgolette.

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

Il tag risultante ha forma `api/v2.4.2` e messaggio `Release api v2.4.2`. La working tree deve essere pulita: nessuna modifica ai file tracciati. I file non tracciati sono ignorati.

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

`--version` e un bump non possono essere usati insieme. Il parsing è strict: valori come `1.2`, `v1.2.3` e `01.2.3` non sono accettati. Se il servizio ha già una release, la versione esplicita deve essere maggiore dell'ultima; con `--force` il tag viene creato comunque, ma non diventa l'ultima release, perché questa è sempre la SemVer più alta.

Prerelease e build metadata sono tag validi (`api/v2.1.0-rc.1`, `api/v1.0.0+build.5`). Una prerelease partecipa all'ordinamento SemVer (`2.0.0` < `2.1.0-rc.1` < `2.1.0`), quindi può essere l'ultima release restituita da `version-number`. Da `2.1.0-rc.1` il bump `patch` produce `2.1.0`, `minor` produce `2.2.0` e `major` produce `3.0.0`.

Il tag rimane locale salvo richiesta esplicita:

```sh
releaser release api patch --push
```

Con `--push`, il comando pubblica soltanto i nuovi tag sul remote configurato, con un unico push atomico: o arrivano tutti sul remote o nessuno. Se la creazione o il push falliscono, i tag locali creati dal comando vengono eliminati, così il comando può essere rilanciato. Non vengono eseguiti fetch, pull, build o deploy impliciti.

### Release in blocco

Per applicare lo stesso bump a tutti e soli i servizi affected:

```sh
releaser release --affected patch
```

Per applicarlo a tutti i servizi configurati, inclusi quelli non affected, occorre confermare esplicitamente con `--force`:

```sh
releaser release --all --force major
```

Il comando valida l'intero batch prima di creare tag. Ogni servizio deve avere una release precedente, perché la versione esplicita non è supportata in modalità bulk. `--dry-run` e `--push` sono disponibili anche per le release in blocco; con `--push` tutti i tag creati vengono pubblicati sul remote configurato in un unico push atomico.

### Repository standard (root)

Per rilasciare un repository standard, non organizzato come monorepo, usa `--root`. In questa modalità `releaser.yml` non è letto:

```sh
releaser release --root patch
releaser release --root --version 1.0.0
```

Il tag predefinito non ha prefix, per esempio `v0.1.5`. Se esiste già una release, il prefix viene dedotto dai tag precedenti; se vengono trovati prefix eterogenei, `--prefix` diventa obbligatorio. Nella deduzione un prefix è valido solo se vuoto o se termina con un separatore (`/`, `-`, `_`, `.`): `release-v1.0.0` ha prefix `release-`, mentre `dev1.0.0` non è considerato un tag di release. Un valore esplicitamente vuoto (`--prefix=''`) seleziona tag senza prefix:

```sh
releaser release --root patch --prefix=''
releaser release --root minor --prefix='release-'
```

Se non ci sono commit con modifiche dopo il tag precedente, la release richiede `--force`. `--dry-run` e `--push` sono supportati; il push usa `origin`. `--root` è mutuamente esclusivo con `--affected` e `--all`.

## Esecuzione di script per servizio

`run` esegue uno script (o un qualsiasi comando) una volta per ogni servizio configurato, in ordine alfabetico, oppure solo per il servizio indicato:

```sh
releaser run build.sh           # una volta per servizio
releaser run api build.sh       # solo per api
releaser run --affected build.sh  # solo per i servizi affected
releaser run api build.sh -- --push latest  # argomenti passati allo script
```

Allo script sono esposte queste variabili d'ambiente:

| Variabile | Valore |
| --- | --- |
| `RELEASER_NAME` | nome del servizio |
| `RELEASER_PATHS` | `paths` separati da spazio |
| `RELEASER_DEPS` | `dependencies` separate da spazio (vuota se assenti) |
| `RELEASER_VERSION` | ultima versione rilasciata, per esempio `2.4.1` (vuota se non disponibile) |
| `RELEASER_TAG` | tag dell'ultima release, per esempio `api/v2.4.1` (vuota se non disponibile) |
| `RELEASER_VAR_<KEY>` | una per ogni chiave di `vars`, in maiuscolo e con `-` sostituito da `_` |

Con la configurazione di esempio, per `api` lo script riceve `RELEASER_VAR_IMAGE=registry.example.com/project/api` e `RELEASER_VAR_DEPLOYMENT=api-production`:

```sh
#!/bin/sh
set -e
for p in $RELEASER_PATHS; do echo "path: $p"; done
docker build -t "$RELEASER_VAR_IMAGE" "services/$RELEASER_NAME"
```

Lo script è eseguito dalla root del repository (`--repo`); se il percorso indicato esiste rispetto alla directory corrente viene eseguito direttamente (deve essere eseguibile e avere uno shebang), altrimenti è cercato nel `PATH`. Stdin, stdout e stderr sono quelli di `releaser`; prima di ogni esecuzione su stderr è stampata la riga `==> <service>`.

L'esecuzione si ferma al primo script che fallisce e `releaser` termina con lo stesso exit code dello script. Prima di eseguire qualsiasi script tutte le variabili vengono validate: una chiave di `vars` che non produce un nome valido (solo lettere, cifre, `_` e `-`), due chiavi che producono lo stesso nome (per esempio `my-var` e `my_var`) o un path contenente spazi sono un errore di configurazione. `RELEASER_VERSION` e `RELEASER_TAG` sono vuote per un servizio mai rilasciato o se `--repo` non è un repository Git: in quel caso `run` funziona comunque, tranne con `--affected`.

## Generazione di file da template

`template` esegue un template Go ([`text/template`](https://pkg.go.dev/text/template)) una volta per ogni servizio configurato, in ordine alfabetico, oppure solo per il servizio indicato:

```sh
releaser template deploy.yaml.tpl                  # tutti i servizi, concatenati su stdout
releaser template api deploy.yaml.tpl > api.yaml   # solo api
releaser template deploy.yaml.tpl -o 'deploy/{{.Name}}.yaml'  # un file per servizio
```

Nel template sono disponibili:

| Campo | Valore |
| --- | --- |
| `.Name` | nome del servizio |
| `.Paths` | lista dei `paths` |
| `.Deps` | lista delle `dependencies` |
| `.Version` | ultima versione rilasciata, per esempio `2.4.1` (vuota se non disponibile) |
| `.Tag` | tag dell'ultima release, per esempio `api/v2.4.1` (vuota se non disponibile) |
| `.Vars` | mappa delle `vars`, con `-` sostituito da `_` nelle chiavi |

Con la configurazione di esempio, il template:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.Vars.deployment}}
spec:
  template:
    spec:
      containers:
        - name: {{.Name}}
          image: {{.Vars.image}}:{{.Version}}
---
```

produce per `api`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-production
spec:
  template:
    spec:
      containers:
        - name: api
          image: registry.example.com/project/api:2.4.1
---
```

Le liste si scorrono con `range`, per esempio `{{range .Paths}}{{.}} {{end}}`. Una chiave mancante in `.Vars` (per esempio un refuso come `{{.Vars.imgae}}`) è un errore; per una variabile facoltativa usa `index`, che restituisce una stringa vuota: `{{with index .Vars "port"}}port: {{.}}{{end}}`.

Nei template un nome dopo il punto non può contenere `-`, quindi nelle chiavi di `.Vars` ogni `-` è sostituito da `_`: la variabile `docker-file` si scrive `{{.Vars.docker_file}}`. Due chiavi che producono lo stesso nome (per esempio `docker-file` e `docker_file`) sono un errore di configurazione.

Senza `-o` l'output di tutti i servizi è scritto su stdout, uno dopo l'altro. Con `-o`/`--output` ogni servizio è scritto in un file; il path di output è a sua volta un template con gli stessi campi e deve essere diverso per ogni servizio (tipicamente contiene `{{.Name}}`). Le directory mancanti vengono create, i file esistenti sovrascritti, e su stdout è stampato il path di ogni file scritto. Il path del template e quello di output sono relativi alla directory corrente.

Tutti i servizi sono renderizzati prima di scrivere: se il template fallisce per un servizio non viene scritto nulla. `.Version` e `.Tag` sono vuoti per un servizio mai rilasciato o se `--repo` non è un repository Git.

## Exit code ed errori

Gli errori sono scritti su stderr con prefisso `ERROR:`. Gli exit code sono:

| Codice | Significato |
| ---: | --- |
| 0 | successo |
| 1 | input non valido, servizio/versione non disponibile o stato non valido |
| 2 | configurazione non leggibile o non valida |
| 3 | repository, history o operazione Git non disponibile |

In una shallow clone occorre rendere disponibili tag e history necessari prima di eseguire il tool; `releaser` non accede automaticamente alla rete. Se la history tra un tag e `HEAD` manca, il comando fallisce con exit code 3 invece di riportare un risultato potenzialmente errato.

## Elenco rapido

```text
releaser config check
releaser status [service] [--verbose]
releaser affected
releaser changes <service>
releaser get-var <service> <key>
releaser run [service] <script> [--affected] [-- args...]
releaser template [service] <file> [-o <path-template>]
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
