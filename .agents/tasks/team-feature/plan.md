# Implementation Plan — Team & Management feature

Add a "Team & Management" feature across the stack: a `team_members` DB table, a Go
`team` backend module (mirroring `merchandising`), a shared `TeamMember` TS type, admin
API-client methods + a `TeamView.vue` admin manager, and a public customer `/team` page
linked from the profile page.

## Design decisions (grounded in the repo)

- **Backend module shape.** `merchandising` keeps everything in `collections.go`
  (holds the `Repository` struct + `NewRepository`) and `banners.go` (per-entity
  queries), with a single `handler.go` exposing `NewHandler` + `RegisterPublic` +
  `RegisterAdmin`. The new `team` module mirrors this: `team.go` defines the
  `Repository` struct, `NewRepository`, the `TeamMember` model and all queries;
  `handler.go` defines `NewHandler`, `RegisterPublic`, `RegisterAdmin`. This matches the
  template exactly rather than inventing model.go/repository.go split files.
- **Migrations.** `migrations/embed.go` uses `//go:embed *.sql` (no explicit file list),
  so new migration files are picked up automatically — only the two SQL files are needed.
  `gen_random_uuid()` is available (pgcrypto enabled in `0001_extensions.up.sql`).
- **Photo upload.** Reuse the existing `uploadHandler` (`POST /admin/uploads` →
  `{ key, url, size }`) exactly as `BannersView.vue` does via `admin.uploadImage(file)`.
  No new upload endpoint. `photo_key` stores the returned key, resolved on the client
  with `resolveMediaUrl(MEDIA_BASE, key)` (same pattern as banners `image_key`).
- **Admin edit scope.** Per the product decision, admins edit ONLY `name` + `photo_key`.
  `role`, `bio`, `sort_order`, `is_active` are seeded system defaults. The update repo
  method and handler accept name + photo_key only and ignore anything else. List +
  update are enough; no create/delete (the 5 rows are seeded) — keeps the surface minimal.
- **Permission.** Reuse the existing `auth.PermBannerManage` for the admin team routes.
  Adding a brand-new permission means seeding/role-map changes across `rbac.go`; the
  product treats team management as the same "merchandising/content" admin capability as
  banners, so reusing `PermBannerManage` avoids touching RBAC seeding and keeps admins who
  can manage banners able to manage the team. (If a reviewer insists on a dedicated
  permission, that is a separate RBAC change — out of scope here.)
- **Public route.** `/team` named `team`, no meta guard (same as `/about`, `/privacy`).
- **Shared type consumption.** `@carlitos/types` is consumed from source
  (`main`/`types` → `./src/index.ts`), so there is NO separate types build step — the
  apps' `vue-tsc` picks up the new interface directly.

## Verification commands (per subsystem)

- Backend: in `/Users/sahilibrahimm/Desktop/Arsh-Project/backend` run
  `go build ./...` then `go vet ./...` — both exit 0.
- Admin web: in `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/admin-web` run
  `npx vue-tsc --noEmit` — exits 0, no errors.
- Customer web: in `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/customer-web` run
  `npx vue-tsc --noEmit` — exits 0, no errors.
- Types package: no build; it is validated transitively by the two `vue-tsc` runs above.

---

- [ ] 1. Add the shared `TeamMember` TS interface.
      Add `export interface TeamMember { id: string; name: string; role: string; bio: string; photo_key: string; sort_order: number; is_active: boolean }` in the Merchandising section of the types index (right after the `Banner` interface). This is first because every other TS file (api-client, admin view, customer view) imports it.
      Files: `/Users/sahilibrahimm/Desktop/Arsh-Project/packages/types/src/index.ts`
      Verify: `npx vue-tsc --noEmit` in `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/admin-web` still passes (type resolves).

- [ ] 2. Create migration 0025 up + down for the `team_members` table with 5 seed rows.
      Up: create table (schema below), the `(is_active, sort_order)` index, and INSERT 5 default rows. Down: drop the table (index drops with it). Numbers follow the existing `0024_*` convention; `embed.go` needs no change (`//go:embed *.sql`).
      Files:
      - `/Users/sahilibrahimm/Desktop/Arsh-Project/backend/migrations/0025_team_members.up.sql`
      - `/Users/sahilibrahimm/Desktop/Arsh-Project/backend/migrations/0025_team_members.down.sql`
      Up contents:
      ```sql
      -- =====================================================================
      -- 0025 Team & Management members
      -- Admin-editable company team roster shown on the customer storefront.
      -- role/bio/sort_order are seeded system defaults; admins edit name + photo.
      -- =====================================================================

      CREATE TABLE IF NOT EXISTS team_members (
          id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
          name        TEXT NOT NULL,
          role        TEXT NOT NULL,
          bio         TEXT NOT NULL DEFAULT '',
          photo_key   TEXT,
          sort_order  INT NOT NULL DEFAULT 0,
          is_active   BOOLEAN NOT NULL DEFAULT TRUE,
          created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
          updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
      );

      CREATE INDEX IF NOT EXISTS idx_team_members_active_sort
          ON team_members (is_active, sort_order);

      INSERT INTO team_members (name, role, bio, photo_key, sort_order) VALUES
          ('Founder Name',   'Founder & Chairman',  'Founder and chairman guiding the company vision.',       NULL, 1),
          ('CEO Name',       'Chief Executive Officer', 'Leads overall strategy and company operations.',      NULL, 2),
          ('CTO Name',       'Chief Technology Officer', 'Heads technology, engineering and product delivery.', NULL, 3),
          ('COO Name',       'Chief Operating Officer', 'Oversees day-to-day operations and execution.',        NULL, 4),
          ('Operations Head','Head of Operations',   'Manages fulfilment, logistics and operational quality.', NULL, 5);
      ```
      Down contents:
      ```sql
      -- Revert 0025 team members.
      DROP TABLE IF EXISTS team_members;
      ```
      Verify: `go build ./...` in `/Users/sahilibrahimm/Desktop/Arsh-Project/backend` (the SQL embeds and compiles into the binary; no syntax check tool, so correctness is confirmed by build + the handler queries in later steps).

- [ ] 3. Create the `team` Go module repository + model in `team.go`.
      Define `package team`; the `TeamMember` struct with JSON tags (`id`, `name`, `role`, `bio`, `photo_key`, `sort_order`, `is_active`); `Repository struct{ pool *pgxpool.Pool }` and `NewRepository(pool *pgxpool.Pool) *Repository`. Methods:
      - `ListActive(ctx) ([]TeamMember, error)` — `SELECT id, name, role, bio, coalesce(photo_key,''), sort_order, is_active FROM team_members WHERE is_active = TRUE ORDER BY sort_order, name` (public storefront list).
      - `AdminList(ctx) ([]TeamMember, error)` — same columns, no WHERE, `ORDER BY sort_order, name` (admin manager list — shows all).
      - `UpdateMember(ctx, id, name, photoKey string) error` — `UPDATE team_members SET name = $2, photo_key = COALESCE(NULLIF($3,''), photo_key), updated_at = now() WHERE id = $1`; return `httpx.NotFound("team_member_not_found", "Team member not found")` when `RowsAffected()==0`. Only name + photo are writable (empty photo = keep existing, mirroring `UpdateBanner`'s image handling).
      Mirror the imports/style of `merchandising/collections.go` (`context`, `pgxpool`, `httpx`).
      Files: `/Users/sahilibrahimm/Desktop/Arsh-Project/backend/internal/team/team.go`
      Verify: `go build ./...` in backend compiles (handler added next step wires it).

- [ ] 4. Create the `team` module HTTP handler in `handler.go`.
      Mirror `merchandising/handler.go`: `Handler struct{ repo *Repository }`, `NewHandler(repo *Repository) *Handler`.
      - `RegisterPublic(r fiber.Router)` → `r.Get("/team", h.listTeam)`.
      - `RegisterAdmin(r fiber.Router)` → `r.Get("/team", auth.RequirePermission(auth.PermBannerManage), h.adminListTeam)` and `r.Put("/team/:id", auth.RequirePermission(auth.PermBannerManage), h.updateTeam)`.
      - `listTeam` → `repo.ListActive`, return `httpx.OK(c, items)`.
      - `adminListTeam` → `repo.AdminList`, return `httpx.OK(c, items)`.
      - `updateTeam` → read `id := c.Params("id")` (400 if empty), parse `struct{ Name string `json:"name"`; PhotoKey string `json:"photo_key"` }` via `c.BodyParser`; 400 `validation_failed` "Name is required" if name blank; call `repo.UpdateMember(ctx, id, body.Name, body.PhotoKey)`; return `httpx.Message(c, "Team member updated")`. Ignore any other body fields (role/bio/sort_order not editable).
      Imports: `github.com/gofiber/fiber/v2`, `github.com/carlitos/backend/internal/auth`, `github.com/carlitos/backend/internal/httpx`.
      Files: `/Users/sahilibrahimm/Desktop/Arsh-Project/backend/internal/team/handler.go`
      Verify: `go build ./...` in backend compiles.

- [ ] 5. Wire the `team` module into the router.
      In `register_modules.go`: add import `"github.com/carlitos/backend/internal/team"`; construct `teamRepo := team.NewRepository(pool)` next to `merchRepo := merchandising.NewRepository(pool)`; register public alongside merchandising public: `team.NewHandler(teamRepo).RegisterPublic(public)`; register admin alongside merchandising admin: `team.NewHandler(teamRepo).RegisterAdmin(admin)`.
      Files: `/Users/sahilibrahimm/Desktop/Arsh-Project/backend/internal/router/register_modules.go`
      Verify: `go build ./...` then `go vet ./...` in `/Users/sahilibrahimm/Desktop/Arsh-Project/backend` — both exit 0.

- [ ] 6. Add `team` API-client endpoints (public + admin).
      In `endpoints.ts`: add `TeamMember` to the `@carlitos/types` import list. In `publicApi` add `team: () => c.get<TeamMember[]>('/public/team')`. In `adminApi` (near `banners`/`updateBanner`) add `team: () => c.get<TeamMember[]>('/admin/team')` and `updateTeamMember: (id: string, body: Record<string, unknown>) => c.put<void>(`/admin/team/${id}`, body)`. The admin app already exposes `admin.uploadImage` for the photo upload — reuse it, no new upload method.
      Files: `/Users/sahilibrahimm/Desktop/Arsh-Project/packages/api-client/src/endpoints.ts`
      Verify: `npx vue-tsc --noEmit` in both `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/admin-web` and `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/customer-web` — both pass.

- [ ] 7. Create the admin `TeamView.vue` manager.
      Model on `BannersView.vue` but simpler (edit-only, no create/delete). Script: import `{ onMounted, ref }`, icons `{ Users, Pencil, X, UploadCloud }` from `lucide-vue-next`, `type { TeamMember } from '@carlitos/types'`, `{ admin, MEDIA_BASE } from '@/api'`, `{ resolveMediaUrl, extractApiError } from '@carlitos/shared'`. State: `members = ref<TeamMember[]>([])`, `loading`, `saving`, `uploading`, `error`, `success`, `editingId = ref<string|null>(null)`, `form = ref({ name: '', photo_key: '', photo_preview: '' })`. `load()` calls `admin.team().catch(() => [])`. `openEdit(m)` fills name + photo_key + preview (`resolveMediaUrl(MEDIA_BASE, m.photo_key)`). `onFileChange` uploads via `admin.uploadImage(file)`, sets `form.photo_key`/preview (reuse banner's upload flow; a plain upload without canvas-normalisation is fine for square avatars). `save()` validates name non-empty, calls `admin.updateTeamMember(editingId, { name, photo_key })`, then `load()`. Template: page header "Team & Management"; a list/grid of cards each showing read-only `role` + `bio`, the photo (or a `Users`/initial fallback), and an Edit button; the edit form exposes ONLY name input + photo upload (role/bio shown read-only, not editable).
      Files: `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/admin-web/src/views/TeamView.vue`
      Verify: `npx vue-tsc --noEmit` in `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/admin-web` passes.

- [ ] 8. Register the admin route + sidebar nav entry for Team.
      Router: add `{ path: '/team', name: 'team', component: () => import('@/views/TeamView.vue'), meta: { requiresAuth: true } }` alongside the other merchandising routes.
      Nav: in `AdminLayout.vue` add `Users2` (or reuse an imported icon such as `Users`) to the Merchandising group links: `{ label: 'Team & Management', to: '/team', icon: Users }` — import the icon in the `lucide-vue-next` import block if not already present (`Users` is already imported).
      Files:
      - `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/admin-web/src/router.ts`
      - `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/admin-web/src/components/AdminLayout.vue`
      Verify: `npx vue-tsc --noEmit` in `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/admin-web` passes.

- [ ] 9. Create the customer `TeamView.vue` public page (mobile-responsive).
      Script: import `{ onMounted, ref }`, `{ User } from 'lucide-vue-next'`, `type { TeamMember } from '@carlitos/types'`, `{ pub, MEDIA_BASE } from '@/api'`, `{ resolveMediaUrl } from '@carlitos/shared'`, `PageBack from '@/components/PageBack.vue'`, and `useSeo` (title "Team & Management"). State `members = ref<TeamMember[]>([])`, `loading = ref(true)`. `onMounted` → `members.value = await pub.team().catch(() => [])`. Template: `<PageBack history />` at top (per requirement), `<h1>` "Team & Management", then a responsive grid `grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3`. Each card uses the app `card` style with accent `#ff3f6c`: a fixed-aspect photo (`aspect-square w-full object-cover` via `resolveMediaUrl(MEDIA_BASE, m.photo_key)`) OR, when `photo_key` is empty, a fallback avatar block with a centered `<User>` icon (mirroring ProfileView's avatar circle); then `name` (font-semibold, text wraps), `role` (accent/small), and `bio` (small `text-stone-500`, wraps). Ensure no overflow at ~375px: `min-w-0`, `truncate` only where appropriate (let name/bio wrap, not truncate).
      Files: `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/customer-web/src/views/TeamView.vue`
      Verify: `npx vue-tsc --noEmit` in `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/customer-web` passes.

- [ ] 10. Register the customer `/team` route and add the profile link.
      Router: add `{ path: '/team', name: 'team', component: () => import('@/views/TeamView.vue'), meta: { title: 'Team & Management' } }` in the legal/info (public) block — no auth guard.
      ProfileView: add `{ to: '/team', label: 'Team & Management' }` to the `legalLinks` array (per the user's instruction it sits in the bottom section of the profile page).
      Files:
      - `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/customer-web/src/router.ts`
      - `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/customer-web/src/views/ProfileView.vue`
      Verify: `npx vue-tsc --noEmit` in `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/customer-web` passes.

- [ ] 11. Final full verification of all subsystems.
      Run every subsystem check and confirm no regressions.
      Files: none (verification only).
      Verify:
      - `/Users/sahilibrahimm/Desktop/Arsh-Project/backend`: `go build ./...` and `go vet ./...` exit 0.
      - `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/admin-web`: `npx vue-tsc --noEmit` exits 0.
      - `/Users/sahilibrahimm/Desktop/Arsh-Project/apps/customer-web`: `npx vue-tsc --noEmit` exits 0.

## Notes / assumptions

- Reusing `auth.PermBannerManage` for team admin routes avoids RBAC seeding changes; if a
  dedicated `team.manage` permission is required, that is a follow-up RBAC task.
- No create/delete admin endpoints: the 5 members are seeded and the product only requires
  editing name + photo. The update endpoint deliberately ignores non-name/photo fields.
- `@carlitos/types` has no build step (source-consumed), so the two `vue-tsc` runs fully
  validate the new shared interface — no separate types package build needed.
- Do not commit; work directly in the repo (no worktree), per task constraints.
