# PostgREST row-level security test stack

This local-only Docker stack exercises PostgreSQL row-level security (RLS),
PostgREST authentication, and JWT-backed API calls used by the dedicated
HyperBricks fixtures. PostgreSQL and PostgREST run in separate containers;
pgAdmin is available for manual inspection.

Published ports bind to `127.0.0.1`. The stack is intended for development and
tests on one machine and must not be exposed as a production service.

## Start the stack manually

From this directory, generate local test credentials and start the containers:

```bash
./setup-env.sh
docker compose up --build -d
```

The generated `.env` is mode `0600`, is ignored by Git, and contains the
PostgreSQL password, JWT signing value, and pgAdmin password. To rotate those
values, remove `.env`, run `docker compose down -v`, and run `./setup-env.sh`
again.

The local services are:

- PostgREST: `http://127.0.0.1:3000`
- pgAdmin: `http://127.0.0.1:15432`

Run the end-to-end RLS exercise without printing JWTs:

```bash
./testscript.sh
```

Stop the stack and delete its test data:

```bash
docker compose down -v
```

## Run through the repository test command

From the repository root:

```bash
./scripts/run_api_fragment_render_tests_docker.sh
```

That script generates process-local credentials, starts only PostgreSQL and
PostgREST, runs the dedicated API component fixtures, and removes the Docker
volumes afterward. It does not write the temporary credentials to disk.
