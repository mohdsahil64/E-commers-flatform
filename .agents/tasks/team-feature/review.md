# Team & Management feature across backend, shared types, and both web apps

Adds a company "Team & Management" roster end to end: a `team_members` table seeded with five leadership rows, a Go `team` module mirroring `merchandising`, a shared `TeamMember` type, admin API-client methods plus a `TeamView.vue` manager, and a public customer `/team` page linked from the profile. The design keeps the admin surface deliberately narrow — admins edit only name and photo; role, bio, sort order, and active flag are seeded system defaults and are never writable. Photo handling reuses the existing `/admin/uploads` handler, and the admin routes reuse `auth.PermBannerManage` rather than introducing a new RBAC permission. The change mirrors existing patterns closely enough that it reads like an extension of the banners feature rather than a new subsystem.

Watch for: the admin routes reuse `PermBannerManage` instead of a dedicated `team.manage` permission (confirmed, deliberate per plan — any admin who can manage banners can manage the team). The migration was not applied against a live Postgres and the photo upload round-trip was not exercised end to end (confirmed from verification notes); both are documented caveats, not code defects.

**Verdict**: APPROVED

## High-level view

The backend module is a faithful mirror of `merchandising`: `team.go` holds the model, `Repository`, and queries; `handler.go` holds `NewHandler`/`RegisterPublic`/`RegisterAdmin`. It is constructed next to `merchRepo` and registered via both `RegisterPublic(public)` and `RegisterAdmin(admin)` in `register_modules.go`, so the wiring matches the established template exactly.

The admin edit surface is correctly constrained. `UpdateMember` writes only `name` and `photo_key` (empty photo preserves the existing one via `COALESCE(NULLIF(...))`, the same trick banners use), and the handler body struct only has `Name` and `PhotoKey` fields — any `role`/`bio`/`sort_order`/`is_active` sent by a client is silently ignored because `BodyParser` has nowhere to put it.

Public reads go through `GET /public/team` returning only active members ordered by `sort_order` then `name`, backed by the `(is_active, sort_order)` index the migration creates. The seeded rows carry NULL photos, which the clients render as a graceful fallback avatar.

The shared `TeamMember` interface sits directly after `Banner` in the same types index and is consumed from source, so both apps' `vue-tsc` runs validate it transitively. The admin `TeamView.vue` mirrors `BannersView.vue` (edit-only, role/bio shown read-only), and the customer `TeamView.vue` renders a 1→2→3 column responsive grid with `PageBack history` and the title "Team & Management", linked from `ProfileView` via `legalLinks`.

The only judgment calls are the reused permission and the two untested-at-build-time caveats (live migration, upload round-trip). Both are called out in the plan and verification as intentional scope decisions, and neither blocks the feature.

<details>
<summary>Issues (2)</summary>

1. **Reused banner permission (non-blocking)** — admin team routes use `auth.PermBannerManage` rather than a dedicated `team.manage` permission; acceptable per the plan's product decision, but if team management should be a distinct capability that is a follow-up RBAC change.
2. **Caveats not exercised at build time (non-blocking)** — the migration was not run against a live Postgres and the photo upload round-trip was not tested end to end; run `migrate up` and verify an upload in a DB-backed environment before release.

</details>

<details>
<summary>Details</summary>

### Backend module mirrors merchandising

`team.go` and `handler.go` follow the `merchandising` template precisely: a `Repository struct{ pool *pgxpool.Pool }` with `NewRepository`, a `TeamMember` model with JSON tags matching the table columns, and a `Handler struct{ repo *Repository }` exposing `NewHandler`, `RegisterPublic`, and `RegisterAdmin`. `register_modules.go` builds `teamRepo := team.NewRepository(pool)` next to `merchRepo`, then registers `team.NewHandler(teamRepo).RegisterPublic(public)` alongside the merchandising public registration and `.RegisterAdmin(admin)` alongside the admin one. Both construction and both registrations are present — nothing is half-wired.

```
GET  /public/team        -> listTeam      -> ListActive (active only, sort_order, name)
GET  /admin/team         -> adminListTeam -> AdminList   (all rows)        [PermBannerManage]
PUT  /admin/team/:id      -> updateTeam    -> UpdateMember (name+photo only) [PermBannerManage]
```

The public group is `v1.Group("/public")`, so the handler's `/team` resolves to `/public/team`, which is exactly what `pub.team()` calls.

### Admin edit scope is actually enforced

The requirement is that admins edit only name and photo. This holds at two layers. The repository `UpdateMember` statement sets only `name` and `photo_key` (`photo_key = COALESCE(NULLIF($3,''), photo_key)` keeps the existing photo when the field is blank). The handler's `updateTeamBody` has only `Name` and `PhotoKey` fields, so `role`/`bio`/`sort_order`/`is_active` in a request body are dropped at parse time. Name-blank requests are rejected with `validation_failed`, and a missing row returns `httpx.NotFound("team_member_not_found", ...)`. No create or delete endpoints exist, matching the "five seeded rows, fixed roster" decision.

### Migration shape and index

`0025_team_members.up.sql` creates the table with the expected columns and defaults (`gen_random_uuid()` PK, `is_active` default true, timestamps), the `idx_team_members_active_sort ON team_members (is_active, sort_order)` index that backs the public query's `WHERE is_active = TRUE ORDER BY sort_order`, and five seed rows with NULL `photo_key` and incrementing `sort_order`. The down migration drops the table (the index drops with it). `migrations/embed.go` uses `//go:embed *.sql`, so the new files are picked up without editing the embed list — consistent with how prior migrations are handled.

### Admin TeamView mirrors BannersView

The admin form exposes name as the only editable input; role is rendered in a disabled field with a "fixed default" hint and bio is shown read-only, so the UI reinforces the server-side constraint rather than relying on it alone. File validation (type + 10 MB cap) runs before `admin.uploadImage`, and save calls `updateTeamMember(id, { name, photo_key })`. The admin route carries `meta: { requiresAuth: true }` and the sidebar entry `{ label: 'Team & Management', to: '/team', icon: Users }` is wired with the already-imported `Users` icon.

### Customer page is public and mobile-responsive

The customer view renders `grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3` (1 column on mobile, 2–3 larger), with an `aspect-square object-cover` photo and a centered `User`-icon fallback when `photo_key` is empty — the state of all five seed rows until an admin uploads photos, so the fallback is the default rendering, not an edge case. Name and bio use `break-words` to avoid overflow at narrow widths. The route is registered with only `meta: { title }` and no `requiresAuth`; the global `beforeEach` guard keys on `to.meta.requiresAuth`, so the page is reachable without login. `ProfileView.vue` adds the `/team` entry to `legalLinks`, in the profile's bottom link section.

### Permission reuse

Admin team routes guard with `auth.PermBannerManage` rather than a new `team.manage` permission. This is a deliberate choice documented in the plan: a dedicated permission would require RBAC seeding and role-map changes, and the product treats team management as the same content/merchandising capability as banners. The consequence — any admin who can manage banners can also edit the team — is the intended behavior, not an oversight. Flagged as informational; if the product later wants team management isolated, that is a separate RBAC task.

### Verification evidence

`verification.md` records `go build ./...` and `go vet ./...` at exit 0 and `npx vue-tsc --noEmit` passing in both web apps; the source-consumed types package is validated transitively by those tsc runs. Two gaps are documented and acknowledged: the migration was not applied to a live Postgres (no DB in the sandbox), and the upload round-trip was not exercised end to end. Both reuse existing, unchanged infrastructure (the migrate runner and the `/admin/uploads` handler banners already depend on), so the risk is low, but a `migrate up` and a single manual upload should be done in a DB-backed environment before release. No automated test was added for the new handler; the merchandising module it mirrors also has no handler-level tests, so this is not a coverage regression, but a `ListActive`/`UpdateMember` test would be a reasonable follow-up.

Not verified: live migration apply, photo upload round-trip, and visual rendering at ~375px — none exercisable in the build sandbox.

### No unrelated changes

Every edited file maps to a plan step, and the `register_modules.go` edit adds only the `team` import, the repo construction, and the two registrations; surrounding merchandising wiring is untouched.

</details>

<details>
<summary>File map</summary>

- `backend/migrations/0025_team_members.up.sql` — new table, `(is_active, sort_order)` index, five seed rows.
- `backend/migrations/0025_team_members.down.sql` — drops the table.
- `backend/internal/team/team.go` — model, `Repository`, `ListActive`/`AdminList`/`UpdateMember`.
- `backend/internal/team/handler.go` — `NewHandler`, `RegisterPublic`/`RegisterAdmin`, name+photo-only update.
- `backend/internal/router/register_modules.go` — team import, repo construction, public + admin registration.
- `packages/types/src/index.ts` — `TeamMember` interface after `Banner`.
- `packages/api-client/src/endpoints.ts` — `pub.team()`, `admin.team()`, `admin.updateTeamMember()`.
- `apps/admin-web/src/views/TeamView.vue` — edit-only manager (name + photo).
- `apps/admin-web/src/router.ts` — `/team` admin route.
- `apps/admin-web/src/components/AdminLayout.vue` — sidebar nav entry.
- `apps/customer-web/src/views/TeamView.vue` — public responsive team page.
- `apps/customer-web/src/router.ts` — public `/team` route (no guard).
- `apps/customer-web/src/views/ProfileView.vue` — `legalLinks` entry.

No git diff was available (the workspace is not a git repository); this review was produced by reading the feature's files directly against the plan and verification evidence.

</details>
