# JWT Utilities

RS256 JWT signing per the Docker Registry v2 Token Authentication specification.

## Components

### TokenService

Loads an RSA private key from a PEM file and signs JWT tokens.

- `NewTokenService(keyPath, issuer, expiration) — loads RSA private key (PKCS8 or PKCS1)
- `GenerateToken(service, accesses, subject) — creates signed JWT with claims required by Docker Registry v2

### TokenResponse

Standard token endpoint response with `token`, `expires_in`, and `issued_at` fields.

### AccessEntry

Represents a single access entry in the JWT `access` array:
```json
{"type": "repository", "name": "community/my-plugin", "actions": ["pull", "push"]}
```

## JWT Claims

| Claim | Value |
|-------|-------|
| iss | configured issuer |
| sub | client identifier |
| aud | registry service name |
| exp | now + configured expiration |
| iat | now |
| nbf | now |
| jti | random UUID |
| access | array of AccessEntry |

## Testing

```bash
go test ./pkg/jwt/...
```
