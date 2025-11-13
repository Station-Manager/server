# Station Manager — Server (High‑level)

This document describes, at a high level, how the server handles logbook registration, API key issuance, request authentication/authorisation, and data integrity for QSO uploads. It mirrors the platform‑wide design used by the `apikey` package and avoids implementation details.

Scope and goals
- Prevent unauthorised modification while storing public‑domain QSO data.
- Each QSO belongs to exactly one logbook.
- Each logbook has its own API key; clients must present that key plus the logbook’s identifier in requests.
- The logging station callsign must match the logbook’s callsign. The contacted station callsign (the QSO "call") is unconstrained relative to the logbook’s callsign.

Identifiers
- uid: an immutable, opaque identifier assigned to each logbook for use in external protocols (APIs, clients). It is stable across metadata changes.
- logbook_id: an internal sequential primary key used for joins and internal references. Requests should use `uid`, and the server resolves it to `logbook_id`.

API keys (concepts)
- Full key format: `prefix.secretHex`.
- prefix: an independent random hex string (e.g., 12–16 hex characters) used for indexed lookup; it is not derived from `secretHex` and leaks no information about it.
- secretHex: 64 hex characters derived from a 32‑byte random secret.
- digest: a server‑stored hash of `secretHex` (e.g., SHA‑512 hex) or an HMAC of that value with a server‑side pepper. Only the digest is stored server‑side; the full key is shown to the client once.

High‑level workflow
1) User creates a logbook in the desktop app.
2) The desktop app registers the logbook with the server.
3) The server issues an API key and generates a logbook `uid`; both are returned to the desktop app over TLS (only once).
4) The desktop app stores the logbook metadata, the full API key, and the `uid` locally.
5) The desktop app uploads QSOs with `Authorization: ApiKey <prefix>.<secretHex>` and the logbook `uid`. The server verifies the key, enforces integrity rules, and records usage.

Server responsibilities (high level)
- Key generation
  - Create a 32‑byte random secret (`secretHex`) and an independent random `prefix`.
  - Compute and store a digest of `secretHex` (optionally peppered via HMAC).
  - Associate the key with the logbook and return only `prefix.secretHex` once, alongside the logbook `uid`.

- Key validation
  - Parse `prefix` and `secretHex` from the Authorization header.
  - Resolve `uid` to the internal logbook.
  - Locate an active, unexpired API key for that logbook and `key_prefix`.
  - Compute a digest of the provided `secretHex` and compare to the stored digest using constant‑time comparison (or HMAC if adopted).
  - On success, authorise the request and update usage metrics (e.g., last used, count).

- Rotation and revocation
  - Keys can be revoked at any time. Operational policy may allow multiple concurrent keys per logbook or enforce at most one active key.
  - On rotation, issue a new key and provide its full value to the client; the client replaces its stored key.

- Data integrity rules for QSO writes
  - Enforce that the logging station callsign equals the logbook’s callsign (where present).
  - The contacted station callsign (`qso.call`) is not required to match the logbook’s callsign.

Client responsibilities (high level)
- Store the full API key and `uid` locally with the logbook metadata; never log the full key.
- Include `Authorization: ApiKey <prefix>.<secretHex>` and the logbook `uid` with write requests.
- Replace the stored key on rotation.

Security notes
- Use TLS end‑to‑end. Treat the full API key as sensitive and display it only once on creation.
- Prefer `uid` as the external logbook identifier rather than names or callsigns.
- Consider HMAC with a server‑side pepper to further harden digest storage; keep the pepper out of the database.

See also
- API key high‑level design: ../apikey/README.md
