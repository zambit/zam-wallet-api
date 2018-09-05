# ZAM Wallet-Api

## Installation

### Requirements

* Configured env with Go >= 1.10
* Installed [glide](https://github.com/Masterminds/glide) utility
* Installed [migrate](https://github.com/golang-migrate/migrate) utility
* Postgresql database
* Installer [ginkgo](https://github.com/onsi/ginkgo) utility (for testes only)

Assumed that all commands are invoked in the root on this project.

### Dependencies

Before build it's required to populate all dependencies, just execute

```bash
glide up
```

and wait until complete.

### Testing

Execute in bash

```bash
ginkgo -r .
```

Because some tests uses database and migrations, don't forget pass actual configuration with env variables:

* `WA_DB_URI` - postgres connection param in url form (like `postgres://user:pass@host:port/database?sslmode=disable`)
* `WA_MIGRATIONS_DIR` - relative or absolute path to migrations directory

### Building

Execute in bash

```bash
go build -o {executable_name} cmd/main/main.go
```

It will produces statically linked executable which depends only on `libc`.

### Migrations

Migrations are implemented via [migrate](https://github.com/golang-migrate/migrate) utility.

Basically last revision can be applied by executing

```bash
migrate -path=db/migrations -database=${YOUR_PORSTRES_URI} up
```

### Configuration

See dedicated docs.

## Running

Whole service consist of this parts:
* `server` - serves web of this service (in case of balancing each proceess must be bound to different ports which may be passed either by command line arg or separate config or env variable see configuration for further details)
* `worker` - do some broker jobs (may be parallized)
* `watcher` - watches blockchain events (each coin need separate process)

All of them is required for

### Command line usage

See help with `{binary name} -- help`