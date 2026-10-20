# Verification — Team & Management feature

First iteration (no `review.json` present). Implemented from the plan, then ran
every subsystem check below. All passed.

## Commands run and results

### Backend — `/Users/sahilibrahimm/Desktop/Arsh-Project/backend`

Note: the default `GOCACHE` (`~/Library/Caches/go-build`) is not writable in
this sandbox, so a workspace-local cache was used:
`export GOCACHE=/Users/sahilibrahimm/Desktop/Arsh-Project/.agents/tasks/team-feature/.gocache`.
This changes only the build-cache location, not the compilation result.

- `go build ./...` → exit 0 (printed `BUILD_OK`)
- `go vet ./...` → exit 0 (printed `VET_OK`)

### Admin web — `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/admin-web`

- `npx vue-tsc --noEmit` → exit 0 (printed `ADMIN_TSC_OK`)

### Customer web — `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/customer-web`

- `npx vue-tsc --noEmit` → exit 0 (printed `CUSTOMER_TSC_OK`)

### Types package — `/Users/sahilibrahimm/Desktop/Arsh-Project/packages/types`

No build step — `@carlitos/types` is consumed from source (`./src/index.ts`).
The new `TeamMember` interface is validated transitively by the two `vue-tsc`
runs above, both of which import it (api-client + the two new views).

## What was implemented

1. DB migration `0025_team_members.up.sql` / `.down.sql` with the `team_members`
   table, the `(is_active, sort_order)` index, and 5 seed rows (Founder &
   Chairman, CEO, CTO, COO, Head of Operations) with NULL `photo_key` and
   incrementing `sort_order`. `migrations/embed.go` uses `//go:embed *.sql`, so
   no change to it was needed.
2. Go `team` module (`backend/internal/team/team.go` + `handler.go`) mirroring
   `merchandising`: `Repository`/`NewRepository`, `ListActive`, `AdminList`,
   `UpdateMember` (name + photo only; empty photo keeps existing, same COALESCE
   pattern as `UpdateBanner`). Public `GET /team` (active, ordered sort_order
   then name); admin `GET /team` + `PUT /team/:id` guarded by
   `auth.PermBannerManage` (same capability as banners). Wired into
   `register_modules.go` the same way as merchandising (build repo, then
   `NewHandler(...).RegisterPublic(public)` / `.RegisterAdmin(admin)`).
3. Shared `TeamMember` interface added to `packages/types/src/index.ts` right
   after `Banner`, and `team`/`updateTeamMember` endpoints added to the
   api-client (public + admin), reusing the existing `admin.uploadImage` for
   photos.
4. Admin `TeamView.vue` (modeled on `BannersView.vue`): lists members with
   read-only role/bio and an editable name + photo upload/replace; route `/team`
   and a "Team & Management" sidebar entry under Merchandising (reusing the
   already-imported `Users` icon).
5. Customer `TeamView.vue`: public `/team` route (no auth guard), fetches
   `GET /team` via `pub.team()`, renders a responsive card grid
   (`grid-cols-1` → `sm:grid-cols-2` → `lg:grid-cols-3`) with a fixed
   `aspect-square object-cover` photo and a graceful `User`-icon avatar
   fallback, accent `#ff3f6c`, `PageBack history`, title "Team & Management".
   Added `{ to: '/team', label: 'Team & Management' }` to ProfileView's
   `legalLinks`.

## Caveats not exercisable at build time

- The migration SQL was not applied against a live Postgres (no DB in the build
  sandbox). Schema correctness is confirmed by compilation + the repository
  queries that read exactly those columns; an actual `migrate up` should be run
  in an environment with the database.
- The image-upload round-trip (admin uploads a photo → stored → resolved on the
  customer page) was not exercised end-to-end; it reuses the existing, unchanged
  `/admin/uploads` handler and the same `resolveMediaUrl(MEDIA_BASE, key)`
  pattern that banners already use in production.
- Mobile responsiveness at ~375px is implemented via the single-column grid and
  wrapping (`break-words`, no fixed-width overflow); not visually verified in a
  browser here.
