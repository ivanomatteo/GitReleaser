# Specifiche aggiornate — Monorepo Microservice Release Tool

## 1. Scopo

Realizzare un tool CLI scritto in **Go** per gestire versioning e release indipendenti di microservizi contenuti in un unico monorepo.

Il tool deve consentire di:

* individuare l’ultima versione rilasciata di ogni microservizio;
* determinare quali servizi hanno modifiche non ancora rilasciate;
* considerare dipendenze condivise, ad esempio moduli `common`;
* calcolare la prossima versione SemVer;
* creare tag Git namespacizzati per servizio;
* esporre facilmente numero versione e tag completo;
* produrre output facilmente utilizzabile da pipeline CI/CD;
* mantenere **Git come unica source of truth del versioning**.

Non deve esistere alcun file `VERSION`.

---

# 2. Source of truth

La versione di un servizio è determinata esclusivamente dai suoi tag Git.

Formato:

```text
<service>/v<semver>
```

Esempio:

```text
scraper-service/v0.1.5
api/v2.4.1
worker/v1.8.3
```

Internamente:

```text
service = scraper-service
version = 0.1.5
tag     = scraper-service/v0.1.5
```

Il prefisso `v` appartiene alla convenzione del tag Git.

Il valore SemVer puro è:

```text
0.1.5
```

---

# 3. SemVer

Il tool deve utilizzare Semantic Versioning 2.0.0:

```text
MAJOR.MINOR.PATCH
```

Esempio:

```text
2.7.4
```

Le operazioni previste sono:

```text
patch
minor
major
```

Esempi:

```text
2.7.4 + patch -> 2.7.5
2.7.4 + minor -> 2.8.0
2.7.4 + major -> 3.0.0
```

Il tool non deve determinare automaticamente il tipo di bump.

La scelta tra `patch`, `minor` e `major` rimane responsabilità dell’utente o della pipeline di release.

Per la prima release massiva dei servizi, il comando:

```text
releaser release --new [--version <semver>]
```

deve creare su `HEAD` il primo tag di ogni servizio configurato che non possiede ancora un tag valido nel proprio namespace. La versione predefinita, quando `--version` non è presente, è `0.1.0`.

Esempio:

```text
releaser release --new --version 0.1.0
```

Con i servizi `api`, `worker` e `web`, se soltanto `api` possiede già un tag valido, devono essere creati:

```text
worker/v0.1.0
web/v0.1.0
```

`api` deve essere ignorato indipendentemente dalla versione del suo tag esistente. Il comando deve essere idempotente: se tutti i servizi hanno già una release valida, non crea tag. `--new` non può essere combinato con un servizio, un bump, `--affected`, `--all`, `--root` o `--force`; può essere usato con `--dry-run` e `--push`.

---

# 4. Libreria SemVer Go

Utilizzare:

```text
github.com/Masterminds/semver/v3
```

La libreria deve essere utilizzata almeno per:

* parsing;
* validazione;
* confronto;
* ordinamento;
* incremento versione.

Per il parsing dei numeri di versione provenienti dai tag utilizzare preferibilmente una modalità strict.

Concettualmente:

```go
semver.StrictNewVersion("1.2.3")
```

deve essere accettato.

Valori come:

```text
1.2
v1.2
01.2.3
foo
```

non devono essere accettati come SemVer valido tramite coercion automatica.

Il prefisso `v` deve essere rimosso dal tag prima di passare il valore al parser SemVer.

Esempio:

```text
api/v2.4.1
      ↓
2.4.1
      ↓
StrictNewVersion(...)
```

---

# 5. Struttura repository

Esempio:

```text
repo/
├── services/
│   ├── api/
│   ├── worker/
│   └── scraper/
│
├── common/
│   ├── auth/
│   ├── logging/
│   └── http/
│
├── tools/
│   └── releaser/
│
└── releaser.yml
```

Il tool non deve assumere questa struttura.

Tutti i path devono essere configurabili.

---

# 6. Configurazione

File predefinito:

```text
releaser.yml
```

Esempio:

```yaml
remote: origin

ignore:
  - docs/**

services:

  api:
    paths:
      - services/api
    dependencies:
      - common/auth
      - common/logging
    vars:
      image: registry.example.com/project/api
      deployment: api-production

  worker:
    paths:
      - services/worker
    dependencies:
      - common/logging

  scraper-service:
    paths:
      - services/scraper
    dependencies:
      - common/http
      - common/logging
```

Ogni servizio deve avere `paths` e può avere:

```text
dependencies
ignore
vars
```

`vars` è una mappa opzionale di chiavi e valori stringa specifica del servizio. I valori non stringa non sono validi.

A livello radice sono ammessi `services`, `ignore` (sezione 31) e `remote`, il remote usato da `--push` (predefinito `origin`, sezione 28). Le chiavi sconosciute sono un errore di configurazione, così che un refuso non disattivi silenziosamente una dipendenza.

Il nome di un servizio deve essere utilizzabile come singolo componente di un ref Git: non può essere vuoto, contenere `/`, spazi, caratteri di controllo, `~ ^ : ? * [ \`, `..` o `@{`, iniziare con `.` o `-`, terminare con `.` o `.lock`.

Il file predefinito è cercato nella directory indicata da `--repo`. Un `--config` relativo passato esplicitamente è risolto rispetto alla directory corrente.

---

# 7. Servizio affected

Un servizio è `affected` quando esiste almeno una modifica non inclusa nella sua ultima release.

Il confronto avviene tra:

```text
<ultimo tag del servizio>
```

e:

```text
HEAD
```

Esempio:

```text
api/v2.4.1..HEAD
```

Non deve essere utilizzato:

```text
HEAD~1..HEAD
```

per determinare lo stato di release.

---

# 8. Baseline indipendente per servizio

Ogni servizio ha una propria baseline.

Esempio:

```text
api     -> api/v2.4.1
worker  -> worker/v1.8.3
scraper -> scraper-service/v0.1.5
```

Il tool deve effettuare logicamente:

```text
git diff api/v2.4.1..HEAD
git diff worker/v1.8.3..HEAD
git diff scraper-service/v0.1.5..HEAD
```

Le baseline non sono globali.

---

# 9. Individuazione ultima release

Per un servizio:

```text
scraper-service
```

cercare:

```text
scraper-service/v*
```

Esempio:

```bash
git tag --list 'scraper-service/v*'
```

Supponendo:

```text
scraper-service/v0.1.3
scraper-service/v0.1.4
scraper-service/v0.1.5
```

l’ultima release è:

```text
scraper-service/v0.1.5
```

L’ordinamento deve avvenire tramite SemVer e non lessicograficamente.

---

# 10. Tag validi

Formato valido:

```text
<service>/v<semver>
```

Esempi validi:

```text
api/v1.0.0
api/v1.10.0
api/v2.0.0
```

Esempi non validi:

```text
api/v1
api/v1.2
api/1.2.3
api/v01.2.3
api/foo
```

Sono validi anche prerelease e build metadata secondo SemVer 2.0.0, per esempio `api/v2.1.0-rc.1` o `api/v1.0.0+build.5`.

Le prerelease partecipano all'ordinamento SemVer: `2.1.0-rc.1` precede `2.1.0` ma segue `2.0.0`. Se è la versione più alta, una prerelease è l'ultima release del servizio e viene restituita da `version-number` e `version-tag`. Il bump da una prerelease segue la libreria SemVer:

```text
2.1.0-rc.1 + patch -> 2.1.0
2.1.0-rc.1 + minor -> 2.2.0
2.1.0-rc.1 + major -> 3.0.0
```

Il build metadata non influisce sull'ordinamento: `1.0.0+a` e `1.0.0` hanno la stessa precedenza.

I tag malformati possono essere ignorati normalmente e segnalati con warning in modalità verbose.

Una futura modalità `--strict-tags` può considerarli errori.

---

# 11. Comando `version-number`

Sintassi:

```bash
releaser version-number <service>
```

Esempio:

```bash
releaser version-number scraper-service
```

Output:

```text
0.1.5
```

Se l’ultimo tag è:

```text
scraper-service/v0.1.5
```

il comando restituisce esclusivamente:

```text
0.1.5
```

Nessun testo aggiuntivo deve essere scritto su `stdout`.

Questo comando è progettato per scripting e CI/CD.

Esempio:

```bash
VERSION=$(releaser version-number scraper-service)
```

---

# 11.1 Comando `get-var`

Sintassi:

```bash
releaser get-var <service> <key>
```

Il comando legge il valore dalla sezione `vars` del servizio e lo scrive da solo su `stdout`, con newline finale consentita. Non richiede che la directory indicata da `--repo` sia un repository Git.

Esempio:

```bash
releaser get-var api image
```

Output:

```text
registry.example.com/project/api
```

Servizio o chiave inesistenti producono un errore e un exit code non zero. Il comando è side-effect free ed è adatto a script e pipeline.

---

# 12. Comando `version-tag`

Sintassi:

```bash
releaser version-tag <service>
```

Esempio:

```bash
releaser version-tag scraper-service
```

Output:

```text
scraper-service/v0.1.5
```

Anche in questo caso `stdout` deve contenere esclusivamente il valore.

---

# 13. Comando `next-version-number`

Sintassi:

```bash
releaser next-version-number <service> <patch|minor|major>
```

Esempio:

```bash
releaser next-version-number scraper-service patch
```

Output:

```text
0.1.6
```

Altri esempi:

```bash
releaser next-version-number scraper-service minor
```

Output:

```text
0.2.0
```

```bash
releaser next-version-number scraper-service major
```

Output:

```text
1.0.0
```

Il comando deve essere side-effect free.

---

# 14. Comando `next-version-tag`

Sintassi:

```bash
releaser next-version-tag <service> <patch|minor|major>
```

Esempio:

```bash
releaser next-version-tag scraper-service patch
```

Output:

```text
scraper-service/v0.1.6
```

Il comando non deve creare il tag.

---

# 15. Rimozione comando `next`

Non deve esistere il comando generico:

```text
releaser next
```

Per ottenere versioni e tag la CLI deve esporre esclusivamente:

```text
version-number
version-tag
next-version-number
next-version-tag
```

Questo evita ambiguità sul formato restituito. L'elenco completo dei comandi è nella sezione 42.

---

# 16. Output dei comandi versione

I quattro comandi:

```text
version-number
version-tag
next-version-number
next-version-tag
```

devono essere primitive da scripting.

Regole:

* valore su `stdout`;
* errori su `stderr`;
* nessuna intestazione;
* nessuna decorazione;
* nessun prefisso descrittivo;
* newline finale consentita;
* `exit 0` in caso di successo;
* exit code non zero in caso di errore.

Esempio:

```bash
IMAGE_TAG=$(releaser version-number api)
```

deve funzionare senza `awk`, `sed`, `grep` o `cut`.

---

# 17. Servizio senza release

Se non esistono tag per un servizio:

```bash
releaser version-number new-service
```

deve fallire.

`stderr`:

```text
ERROR: new-service has no released version
```

Exit code:

```text
1
```

Lo stesso vale per:

```text
version-tag
next-version-number
next-version-tag
```

perché non esiste una versione precedente da incrementare.

Il bootstrap viene gestito tramite release con versione esplicita.

---

# 18. Comando `status`

```bash
releaser status
```

Output:

```text
SERVICE           LAST VERSION    AFFECTED
api               2.4.1           yes
worker            1.8.3           no
scraper-service   0.1.5           yes
```

Singolo servizio:

```bash
releaser status scraper-service
```

---

# 19. Status verbose

```bash
releaser status scraper-service --verbose
```

Esempio:

```text
Service: scraper-service

Last version:
  0.1.5

Last tag:
  scraper-service/v0.1.5

Changed files:

  services/scraper/parser.go
  common/http/client.go

Affected:
  yes
```

---

# 20. Comando `affected`

```bash
releaser affected
```

Output:

```text
api
scraper-service
```

Deve restituire esclusivamente i servizi affected.

---

# 21. Comando `changes`

```bash
releaser changes scraper-service
```

Output:

```text
services/scraper/parser.go
common/http/client.go
```

Deve mostrare esclusivamente i file che rendono affected il servizio.

---

# 22. Comando `plan`

```bash
releaser plan
```

Esempio:

```text
SERVICE           VERSION    AFFECTED    FILES
api               2.4.1      yes         4
worker            1.8.3      no          0
scraper-service   0.1.5      yes         2
```

Supportare:

```bash
releaser plan --format json
```

Esempio:

```json
{
  "services": [
    {
      "name": "scraper-service",
      "lastVersion": "0.1.5",
      "lastTag": "scraper-service/v0.1.5",
      "affected": true,
      "changedFiles": 2
    }
  ]
}
```

---

# 23. Comando `release`

Sintassi:

```bash
releaser release <service> <patch|minor|major>
```

Esempio:

```bash
releaser release scraper-service patch
```

Con ultima release:

```text
scraper-service/v0.1.5
```

calcola:

```text
0.1.6
```

e crea:

```text
scraper-service/v0.1.6
```

sul commit corrente.

Il comando deve verificare lo stato affected del servizio prima del dry-run o della creazione del tag. Se il servizio non è affected deve terminare con exit code `1` e un errore, senza creare alcun tag.

Per consentire esplicitamente una release priva di modifiche rilevanti, supportare:

```bash
releaser release scraper-service patch --force
```

`--force` ignora esclusivamente il controllo affected e quello sulla versione esplicita non maggiore dell'ultima (sezione 24); restano valide tutte le altre verifiche, inclusi validità della versione, unicità del tag e working tree pulita.

## 23.1 Release in blocco

Per rilasciare con lo stesso bump tutti i servizi affected:

```bash
releaser release --affected <patch|minor|major>
```

Per rilasciare tutti i servizi configurati, anche se non affected:

```bash
releaser release --all --force <patch|minor|major>
```

`--affected` e `--all` sono mutuamente esclusivi. `--all` richiede sempre `--force`, così il superamento del controllo affected rimane esplicito. Le modalità `--affected` e `--all` non supportano `--version`: ciascun servizio deve avere una release precedente dalla quale calcolare il bump. L'unica modalità bulk che accetta `--version` è `--new` (sezione 3), che riguarda solo servizi mai rilasciati.

Prima di creare il primo tag, il comando deve validare l'intero batch: configurazione dei servizi, stato della history, bump, presenza di una release precedente, unicità di tutti i nuovi tag e working tree pulita. Un errore di validazione non deve quindi lasciare tag parziali. L'ordine di elaborazione è quello alfabetico dei servizi. `--dry-run` deve mostrare tutti i tag senza modificarli; `--push` deve pubblicare tutti i tag creati sul remote configurato con un unico push atomico (`git push --atomic`). Se la creazione di un tag o il push falliscono, i tag locali creati dal comando devono essere eliminati: il comando non lascia release parziali né in locale né sul remote e può essere rilanciato.

## 23.2 Release della root di un repository standard

Per repository non monorepo, la release opera sull'intera root senza caricare né richiedere `releaser.yml`:

```bash
releaser release --root <patch|minor|major> [--dry-run] [--push]
releaser release --root --version <Initial SemVer> [--dry-run] [--push]
```

Il tag ha forma `<prefix>v<semver>`. Il prefix predefinito viene dedotto dai tag release precedenti; nella deduzione un prefix è valido solo se vuoto o se termina con un separatore (`/`, `-`, `_`, `.`), così che tag come `dev1.0.0` non siano interpretati come release con prefix `de`. In assenza di release il prefix è vuoto e produce, per esempio, `v0.1.5`. `--prefix=''` richiede esplicitamente un prefix vuoto. Se i tag release individuati hanno prefix eterogenei, il comando deve fallire e rendere obbligatorio `--prefix`.

Il bump richiede una release precedente; il bootstrap usa `--version` e una SemVer strict. Come nella sezione 24, una `--version` non maggiore dell'ultima release richiede `--force`. Se non esistono modifiche tra il tag precedente selezionato e `HEAD`, il comando fallisce salvo uso esplicito di `--force`. Restano obbligatorie l'ascendenza del tag, l'unicità del nuovo tag e la working tree pulita. `--dry-run` non crea tag; `--push` pubblica il tag su `origin`.

`--root`, `--affected` e `--all` sono modalità mutuamente esclusive.

---

# 24. Release con versione esplicita

Supportare:

```bash
releaser release new-service --version 1.0.0
```

Serve principalmente per:

* bootstrap;
* migrazione di repository esistenti;
* casi amministrativi.

La versione esplicita deve essere validata tramite SemVer strict.

Se il servizio ha già una release, la versione esplicita deve essere strettamente maggiore dell'ultima; altrimenti il comando fallisce con exit code `1`. Con `--force` il tag viene creato comunque (per esempio per ricostruire un tag mancante nella history), ma non diventa l'ultima release, perché questa è sempre la versione SemVer più alta: non è quindi possibile un "reset" verso una versione inferiore.

---

# 25. Bootstrap

Servizio senza tag:

```text
new-service
```

`status new-service --verbose`, con lo stesso formato della sezione 19:

```text
Service: new-service

Last version:
  none

Last tag:
  none

Changed files:

Affected:
  yes
```

Nella tabella di `status` e `plan` la versione compare come `none`; in `plan --format json` `lastVersion` e `lastTag` sono `null`.

Prima release:

```bash
releaser release new-service --version 0.1.0
```

oppure:

```bash
releaser release new-service --version 1.0.0
```

Da quel momento diventano disponibili:

```text
version-number
version-tag
next-version-number
next-version-tag
release patch
release minor
release major
```

---

# 26. Dry-run

```bash
releaser release scraper-service minor --dry-run
```

Output:

```text
Service: scraper-service
Current version: 0.1.5
Next version: 0.2.0
Tag: scraper-service/v0.2.0

No changes have been made.
```

---

# 27. Creazione tag

Preferire tag annotati.

Esempio:

```text
scraper-service/v0.2.0
```

Messaggio:

```text
Release scraper-service v0.2.0
```

---

# 28. Push tag

Opzionalmente:

```bash
releaser release scraper-service patch --push
```

Remote predefinito:

```text
origin
```

Configurabile tramite la chiave `remote` di `releaser.yml` (sezione 6). `release --root` non legge la configurazione e usa sempre `origin`.

Il tool non deve fare push implicito senza flag.

---

# 29. Common

Esempio:

```text
common/logging
      │
      ├── api
      ├── worker
      └── scraper-service
```

Se cambia:

```text
common/logging/logger.go
```

tutti e tre possono risultare affected.

La verifica deve però essere fatta rispetto alla baseline specifica di ogni servizio.

---

# 30. Esempio baseline indipendenti

Cronologia:

```text
A -- B -- C -- D -- E
     ↑       ↑       ↑
 api/v2.4.1  │      HEAD
          worker/v1.8.3
```

Se `common/logging` è cambiato in `C`:

```text
api
```

può risultare affected perché la modifica è successiva alla sua release.

```text
worker
```

può risultare clean perché la modifica è già inclusa nel tag `worker/v1.8.3`.

Quindi ogni servizio deve essere valutato separatamente.

---

# 31. Ignore

Configurazione globale:

```yaml
ignore:
  - docs/**
  - "**/*.md"
```

Configurazione per servizio:

```yaml
services:
  api:
    paths:
      - services/api

    ignore:
      - services/api/docs/**
```

I pattern sono relativi alla root del repository e supportano:

* `**`: qualsiasi numero di directory, anche nessuna;
* `*`: qualsiasi sequenza di caratteri all'interno di un segmento di path;
* `?`: un singolo carattere all'interno di un segmento di path.

Tutti gli altri caratteri, incluse le parentesi quadre, sono letterali. L'ignore globale e quello del servizio si sommano.

---

# 32. Rename e delete

Il detector deve gestire:

* added;
* modified;
* deleted;
* renamed.

Un file eliminato può rendere affected il servizio.

Per un rename devono essere valutati sia path precedente sia path nuovo. Ciascun path è rilevante se appartiene a `paths` o `dependencies` del servizio e non è ignorato; il servizio è affected se almeno uno dei due è rilevante, e `changes` riporta soltanto i path rilevanti.

I path devono essere letti in modo non ambiguo (per esempio `git diff-tree -z`), così che nomi con caratteri non ASCII, spazi o virgolette non vengano alterati dal quoting di Git.

---

# 33. Working tree

Comandi read-only:

```text
status
affected
changes
plan
version-number
version-tag
next-version-number
next-version-tag
```

devono operare su:

```text
HEAD
```

e ignorare modifiche non committate.

Il comando:

```text
release
```

deve richiedere working tree pulito: nessuna modifica, staged o no, ai file tracciati. I file non tracciati non bloccano la release, perché non possono far parte del tag creato su `HEAD` (per esempio artefatti scaricati in CI).

---

# 34. Detached HEAD

Il tool deve funzionare correttamente in detached HEAD.

Questo è fondamentale per CI/CD.

---

# 35. Shallow clone

Se la history necessaria non è disponibile, il tool deve fallire esplicitamente.

Non deve produrre silenziosamente un risultato `affected` potenzialmente errato.

In particolare, se in una shallow clone un tag non risulta antenato di `HEAD`, l'errore deve indicare la history mancante (exit code `3`) e non un tag divergente.

---

# 36. Nessuna rete implicita

Il tool non deve eseguire automaticamente:

```text
git fetch
git fetch --tags
git pull
git push
```

eccetto `push` quando esplicitamente richiesto tramite flag.

La pipeline è responsabile di fornire history e tag sufficienti.

---

# 37. Exit code

```text
0 = successo
1 = validazione negativa / stato non disponibile
2 = errore configurazione
3 = errore Git
4 = errore interno (riservato, attualmente non utilizzato)
```

Rientrano nel codice `1` anche argomenti o flag CLI non validi, servizio sconosciuto, variabile sconosciuta, bump non valido e servizio senza release. Rientrano nel codice `2` file di configurazione mancante, YAML non valido, chiavi sconosciute e nomi o path non validi. Rientrano nel codice `3` repository non valido, history incompleta e comandi Git falliti, incluso il push.

---

# 38. Tag formatter

Default:

```text
{{service}}/v{{version}}
```

Internamente separare:

```text
service
version
tag
```

Non memorizzare mai:

```text
v0.1.5
```

come valore SemVer.

La `v` è responsabilità esclusiva del formatter del tag.

---

# 39. Architettura Go

Struttura suggerita:

```text
cmd/
└── releaser/
    └── main.go

internal/
├── config/
├── git/
├── version/
├── service/
├── changes/
├── release/
└── output/
```

---

# 40. Package `version`

Il package interno:

```text
internal/version
```

(chiamato `version` e non `semver` per non confondersi con la libreria che incapsula)

deve fare da wrapper alla libreria:

```text
github.com/Masterminds/semver/v3
```

Il resto dell’applicazione non deve dipendere direttamente dalla libreria esterna.

API interna indicativa:

```go
type Version struct {
    value *semver.Version
}

func Parse(value string) (Version, error)
func Compare(a, b Version) int

func (v Version) String() string
func (v Version) BumpPatch() Version
func (v Version) BumpMinor() Version
func (v Version) BumpMajor() Version
```

Questo permette di sostituire la libreria in futuro senza modificare il resto del tool.

---

# 41. Git abstraction

Utilizzare Git installato sul sistema tramite:

```go
os/exec
```

Interfaccia indicativa:

```go
type Git interface {
    Tags(pattern string) ([]string, error)

    Resolve(ref string) (string, error)

    DiffFiles(from, to string) ([]ChangedFile, error)

    IsAncestor(commit, descendant string) (bool, error)

    IsClean() (bool, error)

    IsShallow() (bool, error)

    CreateTag(tag, commit, message string) error

    DeleteTag(tag string) error

    PushTags(remote string, tags ...string) error // push atomico
}
```

---

# 42. CLI

Preferire **Cobra** per i subcommand.

Comandi previsti:

```text
releaser status
releaser affected
releaser changes
releaser get-var

releaser version-number
releaser version-tag

releaser next-version-number
releaser next-version-tag

releaser plan
releaser release

releaser config check
```

Non deve esistere:

```text
releaser next
```

---

# 43. Dipendenze esterne

Dipendenze previste:

```text
Cobra
YAML parser
Masterminds/semver/v3
```

Git viene invocato tramite CLI.

Obiettivo:

```text
singolo binario
nessuna dipendenza runtime oltre a Git
```

---

# 44. CI/CD

Esempio:

```bash
VERSION=$(releaser version-number scraper-service)
```

Docker:

```bash
docker build \
  -t registry.example.com/scraper-service:${VERSION} \
  services/scraper
```

Oppure:

```bash
NEXT_VERSION=$(releaser next-version-number scraper-service patch)
```

Oppure:

```bash
TAG=$(releaser version-tag scraper-service)
```

---

# 45. Nessun build e deploy implicito

Il tool gestisce:

```text
Git
versioning
affected detection
release tagging
```

Non gestisce:

```text
Docker build
Maven
Gradle
npm
Composer
deployment
Kubernetes
Helm
OpenShift
```

Rimane completamente polyglot.

---

# 46. Nessun file VERSION

Non devono essere utilizzati file:

```text
VERSION
version.txt
.release-version
```

o equivalenti.

La versione esiste solo come proprietà della release Git.

Questo elimina problemi di sincronizzazione tra:

```text
file locale
tag Git
artefatto pubblicato
```

---

# 47. Invariante principale

Per ogni servizio:

```text
service
   ↓
trova ultimo <service>/v*
   ↓
parse SemVer
   ↓
ultima release
   ↓
diff tag..HEAD
   ↓
paths + dependencies
   ↓
affected
```

Per ottenere la versione:

```text
version-number
      ↓
0.1.5
```

Per ottenere il tag:

```text
version-tag
      ↓
scraper-service/v0.1.5
```

Per calcolare la prossima versione:

```text
next-version-number scraper-service patch
      ↓
0.1.6
```

Per calcolare il prossimo tag:

```text
next-version-tag scraper-service patch
      ↓
scraper-service/v0.1.6
```

---

# 48. Filosofia finale

Il modello deve rimanere:

```text
Git history
    =
stato del codice

Git tag namespacizzato
    =
release del microservizio

releaser.yml
    =
dependency graph

SemVer
    =
identità e significato della release
```

Un monorepo condivide la storia Git, ma non necessariamente il lifecycle di release.

Ogni servizio mantiene:

* versione indipendente;
* tag indipendenti;
* release indipendenti;
* build indipendente;
* deployment indipendente.

Il tool deve aggiungere il minimo stato possibile e lasciare Git come unica fonte autorevole del versioning.
