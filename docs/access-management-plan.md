# Access Management Plan and Implementation Checklist

This repository now includes a phased access-management approach using local users, cookie sessions, and role-based authorization.

## Implementation checklist

- [x] Add auth domain model (`User`, `Session`, `Role`) and store interface
- [x] Add JSON-backed auth store for local persistence
- [x] Add password hashing/verification (bcrypt)
- [x] Add auth service for bootstrap/admin/user/session lifecycle
- [x] Add middleware for authentication + role authorization
- [x] Wire auth service/config in `main.go`
- [x] Add login/logout/forbidden routes and handlers
- [x] Enforce route method restrictions and role-based subrouters
- [x] Add admin user-management routes and handlers
- [x] Add role-aware template data and navigation guards
- [x] Add auth templates (`login`, `forbidden`, `admin-users`)
- [x] Update README with setup and operational guidance

## Roles and permissions

- `viewer`: browse videos and summaries
- `operator`: viewer + submit/process/delete/debug actions
- `admin`: operator + all `/config/*` and `/admin/*`

## Runtime configuration

- `AUTH_ENABLED` (default: `false`)
- `AUTH_BOOTSTRAP_ADMIN_USER`
- `AUTH_BOOTSTRAP_ADMIN_PASS`
- `AUTH_SESSION_DURATION` (Go duration, e.g. `8h`)
- `AUTH_COOKIE_SECURE` (`true` when served over HTTPS)

## Rollout phases

1. **Foundation (done)**: local auth primitives and route guards.
2. **Operational hardening (next)**: login rate limiting, CSRF, and security headers.
3. **Auditability (next)**: add persistent audit events for admin actions.
4. **SSO future (optional)**: keep current authorization layer, swap identity source to OIDC.
