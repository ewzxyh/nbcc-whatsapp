# NBCC WhatsApp

A Go service that reads a private NBCC Brightspace calendar feed, formats an English agenda, and integrates with WhatsApp groups through Grapnel.

**Sending is disabled by default.** Set up and inspect the service before enabling notifications.

## Behavior

- Checks the calendar every five minutes for event changes.
- The first fetch establishes a baseline without sending a message.
- Ignores regenerated `DTSTAMP` values and event ordering when comparing feeds.
- Lists academic deadlines, availability starts, and availability ends from **today through today + 14 days, inclusive**, using `America/Halifax`.
- Groups the agenda by date and prefixes today's date with **`(TODAY)`**.
- Combines deadline and closing events for the same assessment. General campus events are omitted.
- Handles incoming **`#NBCC`** commands in the configured group by fetching a fresh agenda.
- Persists queued work and event IDs, validates signed webhooks, and uses stable idempotency keys for retries.
- Splits long agendas into messages within Grapnel's 4096 UTF-16 character limit.

When `SEND_ENABLED=false`, notifications are not queued and commands are suppressed. Suppressed commands are not replayed after enabling sending.

Any event change in the feed can trigger a notification, including changes outside the displayed window. The passage of a day alone does not trigger a notification; `#NBCC` always recalculates the current window.

## Setup

Copy `.env.example` to `.env` and fill in your own configuration. Keep this file private.

| Variable | Purpose |
|---|---|
| `NBCC_FEED_URL` | Private Brightspace calendar subscription URL |
| `GRAPNEL_API_KEY` | Grapnel API key with the required scopes |
| `GRAPNEL_INSTANCE_ID` | UUID of your connected WhatsApp instance |
| `GRAPNEL_GROUP_JID` | Exact destination group JID returned by Grapnel |
| `GRAPNEL_GROUP_NAME` | Group name used for discovery when no JID is configured |
| `ADMIN_TOKEN` | Random token of at least 32 characters for administrative endpoints |
| `WEBHOOK_SECRET` | Separate random signing secret of at least 32 characters |
| `SEND_ENABLED` | `false` by default; enables sending only when explicitly set to `true` |
| `POLL_INTERVAL` | Feed polling interval; defaults to `5m`, minimum `1m` |
| `NBCC_BIND_PORT` | Host loopback port; defaults to `8080` |

Generate independent secrets for `ADMIN_TOKEN` and `WEBHOOK_SECRET`, for example with `openssl rand -hex 32`. Do not reuse the Grapnel API key as a signing secret.

The Grapnel key needs `instances:read` and `whatsapp:read` for discovery, and `whatsapp:write` for webhook registration and sending. Query Grapnel for your own instance and group IDs; the repository contains only synthetic test identifiers.

## Docker

```sh
docker compose up -d --build
docker compose logs --tail=50 nbcc
```

The service binds its host port to loopback. Persistent state is stored in the `nbcc-data` volume. Preserve that volume across deployments to retain deduplication state.

Inspect the agenda or discover the configured group without sending:

```sh
docker compose exec nbcc /nbcc -preview
docker compose exec nbcc /nbcc -discover
```

## HTTP API

| Method and path | Purpose |
|---|---|
| `GET /healthz` | Service status and sending indicator |
| `GET /api/preview` | Fresh agenda, date window, and activity count |
| `GET /api/group` | Read the configured group through Grapnel |
| `POST /api/check` | Check for feed changes now; respects `SEND_ENABLED` |
| `POST /webhooks/grapnel` | Receive signed Grapnel group message events |

All `/api/*` endpoints require `Authorization: Bearer` with **`ADMIN_TOKEN`**, not the Grapnel API key.

```sh
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://127.0.0.1:8080/api/preview
```

## Webhook deployment

Expose the service through a public HTTPS reverse proxy on port 443. The `ops/` directory provides Nginx and certificate renewal examples using the placeholder domain **`nbcc.example.com`**. Replace it with your domain and adjust the backend port to match `NBCC_BIND_PORT`.

Preserve the raw request body and `X-Grapnel-*` headers. After HTTPS works, register your callback:

```sh
docker compose exec nbcc /nbcc -register-webhook https://nbcc.example.com/webhooks/grapnel
```

Registration does not enable sending. The command refuses to replace an existing instance webhook.

The receiver validates HMAC SHA-256 over `timestamp + "." + raw_body`, matches the event ID, and rejects timestamps more than five minutes from the current time. Only the configured instance, group, and command are processed.

Grapnel forwards incoming messages from other participants (`fromMe:false`). A `#NBCC` command sent by the connected WhatsApp number itself does not trigger this webhook.

## Enable sending

When ready, set `SEND_ENABLED=true` in your private `.env` and recreate the service:

```sh
docker compose up -d --force-recreate
```

There is no HTTP endpoint that enables sending. To disable it again, restore `SEND_ENABLED=false` and recreate the service.

## Development

Requires Go 1.27. The Docker build runs the tests before compiling the static binary.

```sh
go test ./...
go vet ./...
CGO_ENABLED=0 go build -o nbcc-whatsapp .
```

Tests cover date boundaries, `(TODAY)`, recurrence, daylight saving time, stable feed comparisons, webhook signatures, group isolation, disabled sending, restart deduplication, and partial-message retries. Test IDs and signing credentials are synthetic.

## Private data

`.env`, runtime data, production verification reports, calendar exports, and `private/` are excluded from Git and Docker build contexts. Never commit private feed URLs, API keys, signing secrets, live group IDs, server details, or personal calendar snapshots. The application does not need NBCC browser cookies.

The feed contains published dates, not submission status or grades. Grapnel HTTP 202 indicates acceptance, not confirmed WhatsApp delivery.

[Grapnel documentation](https://grapnel.com.br/docs) · [OpenAPI](https://grapnel.com.br/docs/openapi.json)
