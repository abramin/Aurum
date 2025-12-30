# Aurum

Bootstrap for the Aurum finance workspace backend.

## Local development

1. Start infrastructure dependencies:
   ```sh
   docker compose up -d
   ```
2. Run the Go service entrypoint:
   ```sh
   go run ./cmd/aurum
   ```

The current Go entrypoint simply logs a startup message while the wider architecture described in `prd.md` is implemented.
