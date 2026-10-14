# NBCC WhatsApp

A Go service that reads a private NBCC Brightspace calendar feed, formats an English agenda, publishes a public agenda API and UI, and integrates with a WhatsApp group through Grapnel.

**Sending is disabled by default.** Set up and inspect the service before enabling notifications.

## Behavior

- Checks the calendar every `pollIntervalMinutes` (default 5) for event changes.
- While sending is off, establishes and updates a baseline. Enabling sending queues the current agenda.
- Compares the activities actually displayed, ignoring regenerated timestamps, event order, campus events and changes outside the window. Activities entering or leaving the window at midnight, or reaching their Due deadline during the day, are changes too.
- Lists only academic activities with a future **Due** deadline from **today through today + 14 days, inclusive**, using `America/Halifax`. A deadline that has already passed, including earlier today, is excluded even if access closes later. Availability dates are not used as delivery deadlines; activities without Due are excluded from WhatsApp messages. The public calendar still includes their availability phases.
- Groups the agenda by date and prefixes today's date with **`(TODAY)`**.
- Combines phases only when an assessment identifier is available. Otherwise keeps distinct event UIDs separate. General campus events are omitted; malformed campus events follow the same omission policy as the public agenda, and malformed course activities block sending.
- Uses WhatsApp bold for courses, titles and dates. A near deadline adds, for example, `⚠️ *_14 hours left_*` before its title.
- Handles **`#NBCC`** in the configured group by fetching a fresh agenda. After an accepted agenda, commands from anyone are ignored for 15 minutes; ignored commands are never replayed. A pending response also suppresses additional commands.
- After that cooldown, an unchanged agenda is suppressed if its last part is still among the conversation's five most recent messages, including participants' messages. The provider history is checked before suppression; a failed history lookup is retried rather than assuming the agenda has disappeared.
- Sends deadline reminders at **14 hours** and **4 hours** before **Due**, independently of feed changes and the ordinary cooldown/recent-message suppression. Date-only deadlines use 23:59:59 of that local date. Countdown labels show the remaining time when the message is prepared.
- Reminder stages and command cooldowns survive restarts. After downtime, sends the most urgent applicable reminder, combining simultaneous deadlines into one agenda; expired deadlines are never reminded. A changed due date has its own reminder stages.
- Receives other participants' commands by signed webhook. Polls Grapnel history every minute to also detect commands from the connected number, without replaying commands predating activation. Messages need a provider timestamp for history-based command detection.
- Persists queued work and event IDs, validates signed webhooks, and uses stable idempotency keys for retries.
- Splits long agendas into messages within Grapnel's 4096 UTF-16 character limit.

When sending is disabled, notifications are not queued, pending work is canceled and commands are suppressed. Suppressed commands and canceled message parts are not replayed after enabling. The setting and destination are checked before every message part; an HTTP request already in flight may still complete.

New visible activity content supersedes older pending notifications, including retries and incomplete batches. Every outgoing agenda is revalidated against a fresh feed before sending. Its earliest Due deadline and the day boundary are checked before every part; once either passes, remaining parts are canceled and a new agenda is prepared without expired activities. Retries of unchanged work keep exactly the same body and idempotency key for each part. A request already in flight can still be accepted or delivered after a deadline.

Changing the feed URL compares the new feed with the stored baseline, so a different calendar can trigger one notification at the next check when sending is enabled.

## First run and setup

Before the first account is created, set `ADMIN_EMAIL` in the process environment (with Compose, it can be set in the optional `.env` file). Existing installations keep the e-mail in `admin.json`; no reconfiguration is needed. Start the service, then open the admin area:

1. On the first start (no `admin.json` in the state directory), the log contains exactly one line:
   `admin setup code: XXXX-XXXX-XXXX (use it at /admin within this process lifetime)`.
   With Docker: `docker compose logs nbcc`.
2. Open `https://<your-domain>/admin`, enter the setup code and choose a password. The administrator e-mail is fixed on the server, cannot be changed in the UI or API, and is never returned to unauthenticated visitors.
3. The setup code is valid until the account is created or the process restarts. It is kept only as a SHA-256 digest in memory and is never written to disk.
4. Enter the NBCC feed URL, the Grapnel settings and the other values under **Settings**, then test them with the buttons in the admin area.

Restarting the service before setup prints a new setup code.

If you already run this service, the first start after upgrading imports the values from the existing environment (and from `.env`, if present) into `settings.json`, once. Later changes are made in `/admin`, not in `.env`. See `.env.example`.

## Settings

All application settings are edited in `/admin`. They take effect immediately; no restart is needed.

| Setting | Purpose |
|---|---|
| `nbccFeedUrl` | Private HTTPS Brightspace calendar subscription URL (secret; never shown again after saving) |
| `grapnelApiKey` | Grapnel API key with `instances:read`, `whatsapp:read` and `whatsapp:write` (secret) |
| `grapnelInstanceId` | UUID of the connected WhatsApp instance |
| `grapnelGroupJid` | Exact destination group JID; required before sending can be enabled |
| `grapnelGroupName` | Group name used for discovery when no JID is set; default `NBCC Study Group` |
| `webhookSecret` | Signing secret for `POST /webhooks/grapnel`; at least 32 characters, different from the API key (secret) |
| `publicBaseUrl` | Public HTTPS origin, for example `https://nbcc.example.com`; used to register the webhook |
| `sendEnabled` | `false` by default. Enabling requires the API key, instance ID and group JID |
| `pollIntervalMinutes` | WhatsApp feed check interval, 1 to 1440; default 5 |
| `agendaCacheMinutes` | Public agenda cache lifetime, 1 to 1440; default 60 |
| `courseInstructors` | Map of course code to instructor name, up to 100 entries; shown on public course activities |

Infrastructure values are set in the container environment, not in the admin area: `LISTEN_ADDR` (default `:8080`), `STATE_PATH` (default `data/state.json`; Docker uses `/data/state.json`), `WEB_DIR` (default `web/dist`; Docker uses `/web`), `NBCC_BIND_PORT` (compose host port, default 8080), and `TRUSTED_PROXY_IP` (optional exact Docker gateway address of the local reverse proxy).

## Admin endpoints

All admin endpoints are under `/api/admin`. Responses are JSON with `Cache-Control: no-store`. Errors use `{"error": "<code>"}`; validation errors use `422 {"error":"validation","fields":{...}}`. State-changing requests send the `X-CSRF-Token` header returned by the session endpoint.

| Method and path | Auth | Purpose |
|---|---|---|
| `GET /api/admin/session` | none | `setup`, `login` or `authenticated`, with the CSRF token when authenticated |
| `POST /api/admin/setup` | none (origin checked) | Create the administrator with the setup code and a password |
| `POST /api/admin/login` | none (origin checked) | Sign in; sets the session cookie |
| `POST /api/admin/logout` | session + CSRF | End the session |
| `POST /api/admin/session/extend` | session + CSRF | Reset the 60-minute idle timer |
| `GET /api/admin/status` | session | Agenda cache, omitted events with reasons, WhatsApp queue and configured flags |
| `GET /api/admin/settings` | session | Settings; secrets return only `configured` |
| `PUT /api/admin/settings` | session + CSRF | Partial update of any subset of settings |
| `POST /api/admin/settings/test-feed` | session + CSRF | Fetch the saved feed and report counts; does not change the agenda cache |
| `POST /api/admin/settings/test-grapnel` | session + CSRF | Discover the saved group; never sends |
| `POST /api/admin/webhook/register` | session + CSRF | Register `publicBaseUrl` + `/webhooks/grapnel`; refuses to replace an existing webhook; does not enable sending |
| `GET /api/admin/courses` | session | Courses in the cached feed, with instructors and activity counts |
| `POST /api/admin/check` | session + CSRF | Check the feed now; respects `sendEnabled` |
| `GET /api/admin/preview` | session | WhatsApp text preview; never sends |
| `GET /api/admin/activity` | session | Audit log, newest first, at most 200 entries |
| `POST /api/admin/account/email` | session + CSRF | Refuses e-mail changes with `403 fixed_admin_email` |
| `POST /api/admin/password` | session + CSRF | Change the password; requires the current password; revokes other sessions |
| `POST /api/admin/sessions/revoke-others` | session + CSRF | End every other session |

Removed: `/api/preview`, `/api/group`, `/api/check`, the `ADMIN_TOKEN` bearer token, and the `-register-webhook` CLI flag.

The admin UI is served at `/admin` (and `/admin/...`) from `WEB_DIR/admin/index.html`.

## Sessions, passwords and rate limiting

- Passwords are stored as argon2id hashes with OWASP parameters (19 MiB, 2 iterations, 1 lane). A password must be 12 to 256 characters and must not equal the e-mail address (case-insensitive). Paste is allowed.
- Sessions are server-side. The cookie `nbcc_admin` is `HttpOnly`, `SameSite=Strict`, `Path=/`, and `Secure` over HTTPS. It is a browser-session cookie with no fixed expiry date.
- A session ends after 60 minutes without `session/extend`, and in any case after 12 hours. A new token is issued at each sign-in and at each password change.
- Five failures within 15 minutes (failed sign-ins, failed setup codes, or wrong current passwords) lock that client for 15 minutes. Checks and their results are serialized per IPv4 address or IPv6 `/64`, including concurrent requests. The client address is `X-Real-IP` only when the connection comes from loopback or `TRUSTED_PROXY_IP`; otherwise it is the TCP peer address.
- Wrong sign-in details get the same response whether or not the e-mail exists, and an unknown e-mail is checked against a dummy hash.

## Secrets at rest

- `settings.json` stores the three secret fields (`nbccFeedUrl`, `grapnelApiKey`, `webhookSecret`) encrypted with AES-256-GCM. The field name is bound to each ciphertext.
- The 32-byte key is in `secret.key` in the same directory (mode 0600). **Caveat:** the key lives on the same data volume as the encrypted settings. This protects against a leaked settings file, a copy of `settings.json` alone, or log and screenshot exposure. It does not protect against someone who can read the whole data volume. Protect the volume and its backups accordingly.
- Secret values are never returned to the browser, never logged and never written in plaintext. If `secret.key` is lost, the stored secrets cannot be decrypted, and the service refuses to start until the settings are re-entered in a fresh state directory.

## HTTPS requirement

Session cookies are `Secure` when the request is HTTPS or carries `X-Forwarded-Proto: https`. Use HTTPS for `/admin`. The Nginx example in `ops/` proxies everything, including `/admin` and `/api/admin`, to the service and sets `X-Real-IP`, `X-Forwarded-For` and `X-Forwarded-Proto`. Keep those headers when adapting it.

## Security measures

- Admin area: argon2id passwords, server-side sessions with idle and absolute lifetimes, CSRF tokens on every state-changing request, origin checks, `Cache-Control: no-store`, generic errors, per-client lockout, and an audit log without values or addresses.
- Secrets: encrypted at rest, never returned to the browser, never logged, and compared in constant time.
- Webhooks: HMAC-SHA256 over `timestamp + "." + body`, a five-minute window, and a matching event ID. A webhook is refused when no secret is configured. Only the configured instance, group and `#NBCC` command are processed.
- Public API: no secrets, UIDs, descriptions, links or feed identifiers in responses.

## Admin audit log

`audit.json` records setup, sign-ins and failed sign-ins, sign-outs, password changes, settings updates (names of the changed fields only), webhook registration, and session revocation. Historical e-mail-change entries remain readable. Entries never contain values or IP addresses. The log keeps the last 200 entries and is shown newest first in the admin area.

## Public agenda and UI

`GET /api/agenda` needs no token, never sends messages, and returns JSON in the `America/Moncton` time zone:

- Course assessments are `type: "activity"`, with one item per phase (`available`, `due`, or `closes`). Items for the same assessment share an `activityId`. `course`, `courseTitle`, and `instructor` come from the feed and the admin settings.
- Other feed events are `type: "event"`, including recurring events expanded within the range. Cancelled events are omitted. Meeting-link locations are returned as `null`.
- Output is plain text. Descriptions, links, UIDs, and feed identifiers are never returned; item IDs use HMAC-SHA-256 with the persisted private key, so identifiers cannot be guessed offline. IDs stay stable across restarts with the same key.
- `from` and `to` (`YYYY-MM-DD`, inclusive) default to today minus 14 days and today plus 90 days. A range may span at most 186 days. Invalid ranges return `400`.
- The upstream feed is cached for `agendaCacheMinutes` (default 60). After a failed refresh, the last copy is served for up to 24 hours with its original `updatedAt`. Failed refreshes are retried at most every five minutes. Responses send `Cache-Control: public, max-age=300`. If no usable copy exists, the endpoint returns `502`.
- `omittedEvents` counts campus events left out of this response (see the next section). It is `0` when none.

Course names, assessment titles, due dates, campus events, and instructor names are public through this endpoint. Expose the service only to the audience you intend.

`GET /` serves the public UI from `WEB_DIR`: `index.html` with `no-cache`, and `/assets/*` as immutable. Only regular files are served, and `/api/*` and `/webhooks/*` paths are never served as files. The Docker build compiles the UI in a Node.js 24 stage. For local runs, build it with `npm run build` in `web/` or point `WEB_DIR` at another build.

## Omitted events

A problem with one campus event (not a course assessment) does not stop the agenda. That event is left out, every other item is still shown, and the omission is counted in `omittedEvents` for the requested window. The admin status lists each omitted event with its title, date and reason:

| Reason | Meaning |
|---|---|
| `unsupported_recurrence` | Recurrence rule, `RDATE` or `EXDATE` that the service cannot expand |
| `recurrence_override` | `RECURRENCE-ID` override |
| `high_frequency_recurrence` | `HOURLY`, `MINUTELY` or `SECONDLY` recurrence |
| `invalid_date`, `invalid_end` | Unreadable start or end |
| `invalid_title`, `invalid_location` | Unreadable title or location |
| `duplicate_uid` | A second event with an already-used UID |
| `other` | Anything else |

Problems with course activities (`Available`, `Due`, `Availability Ends`) are still errors: `/api/agenda` returns `502`, and a refresh with such a problem is rejected so the last good copy is kept. An empty calendar is also an error. Omissions include series that started earlier but can affect the requested window, including explicit recurrence dates. Unreadable dates and complex rejected series are counted conservatively when overlap cannot be ruled out; high-frequency series are not expanded just to count warnings. A rejected campus override does not claim its master's UID, so feed order does not hide the master series.

## Webhook deployment

Expose the service through a public HTTPS reverse proxy on port 443. The `ops/` directory provides Nginx and certificate renewal examples using the placeholder domain **`nbcc.example.com`**. Replace it with your domain, set `publicBaseUrl` to the same origin in `/admin`, and adjust the backend port to match `NBCC_BIND_PORT`.

Preserve the raw request body and `X-Grapnel-*` headers. After HTTPS works, register the callback from the admin area (**Webhook**). Registration does not enable sending, and the service refuses to replace an existing instance webhook.

The receiver validates HMAC SHA-256 over `timestamp + "." + raw_body` with the saved `webhookSecret`, matches the event ID, and rejects timestamps more than five minutes from the current time. Only the configured instance, group and command are processed. Changing `webhookSecret` takes effect immediately, so update the registration with Grapnel as well if the secret changes.

Grapnel forwards incoming messages from other participants (`fromMe:false`). Commands from the connected number are detected through the group-history API within about one minute, subject to provider history availability.

## Enable sending

In the admin area, turn on `sendEnabled` under **Settings**. The service refuses to enable it until the Grapnel API key, instance ID and group JID are configured. Turn it off in the same place to stop notifications; no restart is needed.

## Docker

```sh
docker compose up -d --build
docker compose logs --tail=50 nbcc
```

The `.env` file is optional: `env_file` is marked `required: false`, so the stack starts without it. The service binds its host port to loopback. Persistent state, including the encrypted settings and `secret.key`, is stored in the `nbcc-data` volume. Preserve that volume across deployments.

Inspect the agenda or discover the configured group without sending:

```sh
docker compose exec nbcc /nbcc -preview
docker compose exec nbcc /nbcc -discover
```

`-env PATH` selects the optional legacy seed file (default `.env`). It is read only when `settings.json` does not exist yet.

## Development

Requires Go 1.27. The Docker build runs the tests before compiling the static binary.

```sh
go test ./...
go vet ./...
CGO_ENABLED=0 go build -o nbcc-whatsapp .
```

For a local run, set `STATE_PATH` to a scratch directory (the default `data/state.json` is relative to the working directory) and `LISTEN_ADDR=127.0.0.1:8080`. For the UI, run `cd web && npm install && npm run dev`; Vite proxies `/api` to the service.

Tests cover date boundaries, `(TODAY)`, recurrence, daylight saving time, stable feed comparisons, webhook signatures and secret rotation, group isolation, disabled sending, restart deduplication, partial-message retries, setup and sign-in, concurrent lockout and IPv6 prefixes, fixed administrator identity, session lifetimes, CSRF and origin checks, password changes, settings validation, encryption at rest, keyed public IDs, the legacy seed, omitted series and override order, static routing, calendar contrast and secret-field accessibility. Test identifiers and secrets are synthetic.

## Credits

Icons are from [Solar Icon Set](https://github.com/480-Design/Solar-Icon-Set) by 480 Design, licensed under CC BY 4.0, through `@solar-icons/react` (MIT).

## Private data

`.env`, runtime data (`data/`, including `secret.key`), production verification reports, calendar exports, and `private/` are excluded from Git and Docker build contexts. Never commit private feed URLs, API keys, signing secrets, live group IDs, server details, or personal calendar snapshots. The application does not need NBCC browser cookies.

The feed contains published dates, not submission status or grades. Grapnel HTTP 202 indicates acceptance, not confirmed WhatsApp delivery.

[Grapnel documentation](https://grapnel.com.br/docs) · [OpenAPI](https://grapnel.com.br/docs/openapi.json)
