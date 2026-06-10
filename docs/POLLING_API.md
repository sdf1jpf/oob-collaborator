# Polling API (Burp-Style)

Custom OOB polling endpoint compatible with the general Collaborator interaction model. This is **not** the proprietary encrypted Burp private Collaborator protocol.

## Authentication

Provide the shared poll token via either header:

```
X-Collaborator-Token: <POLL_TOKEN>
```

or

```
Authorization: Bearer <POLL_TOKEN>
```

## Endpoints

### Health Check

```
GET /bi/health
```

Response `200`:
```json
{ "status": "ok" }
```

### Poll Interactions

```
GET /bi/b
```

Returns all undelivered interactions and marks them as delivered (sets `delivered_at`). Long-term records remain in PostgreSQL.

Response `200`:
```json
{
  "interactions": [
    {
      "interactionId": "uuid",
      "interactionType": "dns|http|smtp",
      "protocol": "DNS|HTTP|SMTP",
      "sourceIp": "1.2.3.4",
      "timeStamp": "2026-06-10T12:00:00.000000000Z",
      "host": "token123.yourdomain.com",
      "rawData": { ... }
    }
  ]
}
```

## Semantics

- Interactions are **never deleted** from the database upon poll delivery.
- `delivered_at` is set when an interaction is successfully returned to a poller.
- Subsequent polls return only interactions where `delivered_at IS NULL`.
- Dashboard queries always show full history regardless of delivery state.

## Example

```bash
curl -H "X-Collaborator-Token: changeme-poll-token" \
  https://yourdomain.com/bi/b
```
