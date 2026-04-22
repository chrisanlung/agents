# Design System

_Owned by `ui-ux-expert`. Consumed by `nextjs-expert` and `flutter-expert`._

## Design tokens

### Spacing scale (4 base)
`4, 8, 12, 16, 24, 32, 48, 64, 96`

### Type scale
| Token | Size / line-height | Usage |
| --- | --- | --- |
| `display-lg` |  |  |
| `heading-md` |  |  |
| `body-md` |  |  |
| `caption` |  |  |

### Color (semantic roles)
| Token | Light | Dark | Usage |
| --- | --- | --- | --- |
| `bg-surface` |  |  |  |
| `bg-elevated` |  |  |  |
| `text-primary` |  |  |  |
| `text-muted` |  |  |  |
| `border-subtle` |  |  |  |
| `accent` |  |  |  |
| `danger` |  |  |  |
| `success` |  |  |  |

### Radius
`0, 4, 8, 12, 16, 999`

### Motion
| Token | Duration | Easing | Usage |
| --- | --- | --- | --- |
| `fast` | 150ms | ease-out | Hover, fade |
| `medium` | 225ms | ease-out | Modal, drawer |

## Component inventory

### shadcn/ui primitives (vendored into each web app under `components/ui/`)
- [x] Button (primary, secondary, ghost, outline, link variants) — `components/ui/button.tsx`
- [x] Input — `components/ui/input.tsx`
- [x] Label — `components/ui/label.tsx`
- [x] Card, CardHeader, CardContent, CardFooter, CardTitle, CardDescription — `components/ui/card.tsx`
- [x] Form (RHF-wired: Form, FormField, FormItem, FormLabel, FormControl, FormMessage) — `components/ui/form.tsx`
- [x] Toaster (sonner wrapper) — `components/ui/sonner.tsx`
- [ ] Input, Textarea, Select (Textarea/Select still needed)
- [ ] Checkbox, Radio, Switch
- [ ] Modal, Drawer
- [ ] Table, Pagination
- [ ] Nav, Tabs
- [ ] Empty / Loading / Error states

### App-level composed components (Phase 2 login + dashboard)
- [x] `LoginForm` — email/password/tenant-slug form wired to Server Action (all three portals)
- [x] `SignOutButton` — client component wrapping logout Server Action
- [x] `ProfileField` — key/value display for user profile data
- [x] `DashboardLoading` — skeleton loading state for dashboard
- [x] `MustChangePasswordBanner` — yellow alert banner for password rotation gate (inline in dashboard page)

**Token additions made in `app/globals.css` (flagged for `ui-ux-expert` review):**
- `--ring`: per-portal focus ring colour (indigo/teal/amber). Not yet in `DESIGN_SYSTEM.md` token table.

## Screen flows
_List and link Mermaid flow diagrams per major user journey._

### Phase 2 — Login flow (all three portals)
```
/ → cookie present? → /dashboard (Server Component fetches /auth/me)
                  ↓ no cookie
               /login → LoginForm (RHF + Server Action)
                           ↓ POST /auth/login
                           ↓ role check (JWT decode, server-side)
                           ↓ pass → set HttpOnly cookies → redirect /dashboard
                           ↓ fail → toast (role mismatch / bad credentials / rate limit)
```

## Accessibility baseline
- Contrast ≥ 4.5:1 text, 3:1 UI
- Focus visible on all interactive elements
- Keyboard navigable, logical tab order
- `prefers-reduced-motion` respected
- Target size ≥ 24×24 px (44×44 on touch)

## Open questions
-
