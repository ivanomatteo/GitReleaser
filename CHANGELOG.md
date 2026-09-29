# Changelog

Tutte le modifiche rilevanti del progetto sono documentate in questo file.

Il formato segue [Keep a Changelog](https://keepachangelog.com/it-IT/1.1.0/) e il progetto adotta il [Semantic Versioning](https://semver.org/lang/it/).

## [Unreleased]

### Aggiunto

- Comando `run [service] <script> [-- args...]` che esegue uno script per ogni servizio (o solo per quello indicato, o con `--affected` solo per i servizi affected) esponendo `RELEASER_NAME`, `RELEASER_PATHS`, `RELEASER_DEPS`, `RELEASER_VERSION`, `RELEASER_TAG` (vuote se non disponibili) e `RELEASER_VAR_<KEY>`. Si ferma al primo errore propagando l'exit code dello script.
- Comando `template [service] <file> [-o <path>]` che esegue un template Go per ogni servizio (o solo per quello indicato) con i campi `.Name`, `.Paths`, `.Deps`, `.Version`, `.Tag` e `.Vars`. L'output va su stdout o, con `-o`, in un file per servizio il cui path è a sua volta un template.

### Modificato

- Richiesto Go 1.27 o successivo; aggiornate le dipendenze (`Masterminds/semver/v3` 3.5.0, `spf13/cobra` 1.10.2, `spf13/pflag` 1.0.10).
- Il `releaser.yml` predefinito è cercato nella directory indicata da `--repo` invece che nella directory corrente. Un `--config` relativo passato esplicitamente resta relativo alla directory corrente.
- Le chiavi sconosciute in `releaser.yml` sono un errore di configurazione (exit code 2): un refuso come `dependecies` non disattiva più silenziosamente le dipendenze.
- I nomi dei servizi devono essere validi come componente di un tag Git (niente spazi, `~ ^ : ? * [ \`, `..`, `@{`, né inizio con `.`/`-` o fine con `.`/`.lock`).
- `release --version` rifiuta una versione non maggiore dell'ultima release; `--force` permette di creare comunque il tag, che però non diventa l'ultima release.
- I file non tracciati non bloccano più la release: la working tree è considerata pulita se non ci sono modifiche ai file tracciati.
- Con `--push` tutti i tag creati sono pubblicati con un unico `git push --atomic`. Se la creazione o il push falliscono, i tag locali creati dal comando vengono eliminati, così il comando può essere rilanciato.
- `release <service>` senza bump né `--version` segnala subito l'argomento mancante invece dell'errore "not affected".

### Corretto

- File con caratteri non ASCII, spazi o virgolette nel nome non venivano riconosciuti come modifiche del servizio, a causa del quoting dei path di `git diff`. Il rilevamento usa ora `git diff-tree -z`, indipendente dalla configurazione Git dell'utente.
- In una shallow clone la history mancante era segnalata come "release tag is not an ancestor of HEAD" (exit code 1); ora è riportata come history incompleta (exit code 3), anche con `--root`.
- `release --root` deduceva prefix errati da tag come `dev1.0.0` (prefix `de`). Un prefix dedotto deve ora essere vuoto o terminare con `/`, `-`, `_` o `.`.
- `changes` riportava per un rename anche il path esterno al servizio; ora elenca solo i path rilevanti.
- Un errore nel push di una release bulk poteva lasciare tag pubblicati solo in parte.

### Documentazione

- SPECS.md allineata all'implementazione: package `internal/version`, chiavi `remote` e `ignore`, `paths` obbligatorio, formato di `status --verbose` per i servizi non rilasciati, eccezione `--new` per `--version` in bulk, exit code, sintassi dei glob di `ignore`, gestione dei rename, prerelease e build metadata.
- README aggiornato con risoluzione della configurazione, validazione, push atomico, regola sul prefix di `--root`, prerelease e working tree.

## [0.1.3] - 2026-08-20

### Aggiunto

- Sezione `vars` per servizio in `releaser.yml`: mappa di chiavi e valori stringa.
- Comando `get-var <service> <key>`, che stampa solo il valore ed è utilizzabile anche fuori da un repository Git.

## [0.1.2] - 2026-08-18

### Aggiunto

- `release --new [--version <semver>]`: crea su `HEAD` il primo tag (predefinito `0.1.0`) per ogni servizio configurato che non ne ha ancora uno. Il comando è idempotente.

### Documentazione

- README esteso con motivazioni, installazione, configurazione ed esempi d'uso.

## [0.1.1] - 2026-08-12

### Aggiunto

- Binari precompilati per macOS (`darwin/amd64` e `darwin/arm64`).

## [0.1.0] - 2026-08-12

Prima release.

### Aggiunto

- Versioning indipendente dei servizi di un monorepo tramite tag Git `<service>/v<semver>`, con parsing SemVer strict e senza file `VERSION`.
- Configurazione `releaser.yml` con `paths`, `dependencies` e `ignore` per servizio, `ignore` globale e `remote`; comando `config check`.
- Comandi di interrogazione `status [service] [--verbose]`, `affected`, `changes <service>` e `plan [--format json]`, con baseline indipendente per servizio.
- Primitive per scripting `version-number`, `version-tag`, `next-version-number` e `next-version-tag`.
- `release <service> <patch|minor|major>` e `release <service> --version <semver>`, con tag annotati, `--dry-run` e `--push`.
- Controllo affected in `release`, disattivabile con `--force`.
- Release in blocco con `release --affected <bump>` e `release --all --force <bump>`, validate interamente prima di creare tag.
- `release --root` per repository standard, senza configurazione, con deduzione del prefix dei tag e `--prefix`.
- Exit code distinti per errori di validazione (1), configurazione (2) e Git (3).
- Build multipiattaforma con GoReleaser e workflow GitHub Actions per la pubblicazione delle release.

[Unreleased]: https://github.com/ivanomatteo/GitReleaser/compare/v0.1.3...HEAD
[0.1.3]: https://github.com/ivanomatteo/GitReleaser/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/ivanomatteo/GitReleaser/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/ivanomatteo/GitReleaser/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/ivanomatteo/GitReleaser/releases/tag/v0.1.0
