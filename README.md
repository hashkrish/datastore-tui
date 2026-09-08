# datastore-tui

A terminal UI for browsing and editing [Google Cloud Datastore](https://cloud.google.com/datastore)
data over its REST API, with vim-style keybindings.

Browsing uses a ranger-style, three-column Miller-column layout
(Namespace → Kind → Entity) plus a live preview pane of the highlighted
entity's properties. Opening an entity switches to a dedicated detail view
for recursively navigating and editing its (possibly nested) properties —
arrays, embedded entities, geopoints, key references, and so on each get
their own typed edit form.

Works against the local [Datastore emulator](https://cloud.google.com/datastore/docs/tools/datastore-emulator)
(no authentication) or a real GCP project, authenticated via Application
Default Credentials.

## Installation

Requires Go 1.25+.

```sh
go install github.com/krishnan/datastore-tui@latest
```

Or build from source:

```sh
git clone https://github.com/krishnan/datastore-tui.git
cd datastore-tui
go build -o datastore-tui .
```

Prebuilt binaries for tagged releases are attached to each
[GitHub release](https://github.com/krishnan/datastore-tui/releases).

## Running against the emulator

```sh
gcloud components install cloud-datastore-emulator   # first time only
gcloud beta emulators datastore start --no-store-on-disk --host-port=localhost:8081
```

In another terminal:

```sh
export DATASTORE_EMULATOR_HOST=localhost:8081
datastore-tui -project my-test-project
```

`-project` can be any string the emulator accepts unauthenticated; it does
not need to be a real GCP project.

## Running against a real GCP project

If `-endpoint` isn't passed and `DATASTORE_EMULATOR_HOST` isn't set,
`datastore-tui` targets the real Cloud Datastore API
(`https://datastore.googleapis.com`) and authenticates using [Application
Default Credentials](https://cloud.google.com/docs/authentication/application-default-credentials)
(ADC) — the same resolution `gcloud` and the official client libraries use.
Set up credentials with one of:

```sh
gcloud auth application-default login
```

or, for a service account key:

```sh
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json
```

The identity needs the `roles/datastore.user` (or `roles/datastore.owner`)
IAM role on the target project. Then run:

```sh
datastore-tui -project my-real-project
```

The project ID can be omitted if ADC itself carries one (e.g. a service
account key's `project_id`, or `gcloud`'s configured project) — otherwise
pass `-project` or set `GOOGLE_CLOUD_PROJECT`/`DATASTORE_PROJECT_ID`.

The first time you point `datastore-tui` at a real (especially production)
project, add `-read-only` to browse safely before trusting yourself with
edits:

```sh
datastore-tui -project my-real-project -read-only
```

## Configuration

| Flag         | Env var                                       | Default                                            |
|--------------|------------------------------------------------|------------------------------------------------------|
| `-project`   | `GOOGLE_CLOUD_PROJECT`, `DATASTORE_PROJECT_ID` | `test-project` (emulator) / from ADC (real GCP)     |
| `-endpoint`  | `DATASTORE_EMULATOR_HOST` (as `http://<host>`) | `http://localhost:8081` (emulator) / `https://datastore.googleapis.com` (real GCP) |
| `-read-only` | —                                              | `false`                                             |

Setting `-endpoint` or `DATASTORE_EMULATOR_HOST` selects local emulator mode
(no authentication); leaving both unset selects real-GCP mode (ADC-authenticated).
`-read-only` disables every mutating action (new/delete entity,
retype/add/delete property, save) — a `[read-only]` marker shows in the
status line, and blocked actions report why nothing happened.

`datastore-tui --version` prints build info; `datastore-tui -h` prints flag usage.

## Keybindings

### Browse mode (Namespace / Kind / Entity columns)

| Key             | Action                          |
|-----------------|----------------------------------|
| `j`/`k`, `↓`/`↑` | move                             |
| `ctrl+d`/`ctrl+u`| half page down/up                |
| `h`/`l`, `←`/`→` | back / drill into the next column |
| `gg` / `G`       | jump to top / bottom of the column |
| `/`              | filter the focused column        |
| `enter`          | open the selected entity         |
| `o`              | create a new entity (Entity column) |
| `dd`             | delete the selected entity (Entity column, with confirmation) |
| `R`              | refresh the focused column       |
| `?`              | help                              |
| `q`, `ctrl+c`    | quit                              |

The rightmost pane always previews the highlighted entity's properties, so
you can see its fields before opening it for edit.

### Detail mode (viewing/editing an entity)

| Key             | Action                          |
|-----------------|----------------------------------|
| `j`/`k`          | move between properties          |
| `ctrl+d`/`ctrl+u`| half page down/up                |
| `l`/`enter`      | expand an array/embedded entity, or edit a scalar |
| `t`              | change the selected property's data type (works on any property, including `null`) |
| `h`/`esc`        | back out one level, or return to browse |
| `/`              | filter the current scope's properties |
| `o`              | add an array item (when viewing an array) |
| `dd`             | delete an array item (when viewing an array, with confirmation) |
| `ctrl+s`         | save pending edits (a single batched commit) |
| `q`/`esc`        | back to browse (prompts if there are unsaved edits) |

## Architecture

| Package                | Responsibility |
|-------------------------|-----------------|
| `datastore/model`       | Typed `Key`/`Entity`/`Value` domain types and their Datastore REST API v1 JSON (de)serialization |
| `datastore/client`      | Thin REST client: `lookup`, `runQuery`, `commit`, `beginTransaction`/`rollback` |
| `datastore/query`       | Namespace/Kind listing and paginated entity queries, built on `datastore/client` |
| `ui/nav`                | Miller-column navigation state machine and the detail-view property-path breadcrumb |
| `ui/panes`              | Rendering for the browse columns/preview and the entity detail view, plus path-addressed get/set/append/remove helpers for mutating an entity in place |
| `ui/edit`               | [huh](https://github.com/charmbracelet/huh)-based edit forms per property type, and the save/delete commit helpers |
| `ui/keymap`             | Centralized vim-style `key.Binding` sets per mode, plus the two-key chord tracker for `gg`/`dd` |
| `app`                   | Top-level [Bubble Tea](https://github.com/charmbracelet/bubbletea) model wiring everything together |
| `config`                | CLI flag/env var resolution for the Datastore endpoint and project ID |

Built with the [Charm](https://charm.sh/) TUI stack: Bubble Tea, Bubbles,
Lip Gloss, and Huh.

## Development

```sh
make build   # go build -o datastore-tui .
make test    # go test ./...
make vet     # go vet ./...
make lint    # golangci-lint run (requires golangci-lint installed)
make run     # build and run against DATASTORE_EMULATOR_HOST
```

See `make help` for the full target list.
