# vali

A lightweight, high-performance domain validation microservice for **Caddy**'s `on_demand_tls` ask endpoint.

`vali` validates incoming domain requests against static configurations, Redis key-value stores, and MariaDB/MySQL database records to allow automated SSL certificate issuance on the fly.

## Quick Start

Run `vali` via Docker/Podman:

```bash
docker run -d \
  -p 8080:8080 \
  -e REDIS_URL="redis:6379" \
  -e VALI_DOMAINS="example.com,*.example.net" \
  --name vali \
  ghcr.io/your-user/vali:latest
```

## Endpoints

* **`GET /vali/{domain}`** or **`GET /vali/?domain={domain}`**
Returns `200 OK` if the domain is permitted, or `404 Not Found` if forbidden.
* **`GET /healthz`**
Health check endpoint for container orchestrators. Returns `200 OK` when dependencies are healthy.

## Environment Variables

| Variable | Default | Description |
| --- | --- | --- |
| `VALI_PORT` | `8080` | Port for HTTP server |
| `VALI_DOMAINS` | *None* | Comma-separated list of static domains (supports `*` wildcards) |
| `VALI_DOMAINS_FILE` | `vali_domains.json` | Path to JSON file containing array of static domains |
| `VALI_USE_REDIS` | `true` | Set to `false` to disable Redis dependency |
| `REDIS_URL` | `localhost:6379` | Redis address (`host:port` or `redis://...` URL) |
| `VALI_USE_MARIADB` | `false` | Enable SQL query lookup |
| `DB_HOST` | `localhost` | MariaDB / MySQL server address |
| `DB_PORT` | `3306` | MariaDB / MySQL server port |
| `DB_USERNAME` | *None* | Database user |
| `DB_PASSWORD` | *None* | Database password |
| `DB_DATABASE` | *None* | Database name |

## Caddy Integration

Add `vali` as the `ask` endpoint in your `Caddyfile`:

```caddy
{
    on_demand_tls {
        ask http://vali:8080/vali/
    }
}

https:// {
    tls {
        on_demand
    }

    reverse_proxy app:3000
}

```

## Static Domains File (`vali_domains.json`)

Optional JSON array mounted inside the container:

```json
[
  "example.org",
  "*.internal.net"
]

```
