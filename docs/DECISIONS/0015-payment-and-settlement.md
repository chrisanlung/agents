# ADR 0015 — Payment Provider Abstraction + Settlement Lifecycle (Phase 6)

- **Status:** Accepted
- **Date:** 2026-04-30
- **Deciders:** owner (2026-04-30), orchestrator
- **Supersedes:** ADR 0014 §3.3 (Midtrans-specific payment) — generic abstraction now; iPaymu is the first concrete provider.

---

## 1. Context

Phase 5 shipped a dummy payment adapter that instant-confirms bookings. Owner (2026-04-30) decided to move to a real-but-simple QRIS-based flow via **iPaymu** (Indonesian payment aggregator, indie-friendly, single QRIS channel).

Critical realisation during design discussion: **payment ≠ settlement**. iPaymu holds funds in escrow after the customer pays; the money lands in Lustia's bank account ~H+1 (next business day) via daily settlement. Then Lustia disburses to tenants on a separate cycle (weekly).

Phase 6 builds the full money-lifecycle backbone — not just the customer-facing QR flow. Customer experience stops at `paid`; tenant accounting cares about `settled` and `disbursed`.

## 2. Decisions

### 2.1 Three-tier money lifecycle

```
  Customer scans QR + pays
            │
            ▼
  ┌───────────────────────────────────────────────────────────────┐
  │ T+0 (instant)  iPaymu webhook → backend                        │
  │                booking.status: pending_payment → paid          │
  │                payment_transaction.status: awaiting → paid     │
  │                Customer receives 8-char code                   │
  │                Money sits in iPaymu ESCROW                     │
  └───────────────────────────────────────────────────────────────┘
            │
            ▼
  ┌───────────────────────────────────────────────────────────────┐
  │ H+1            iPaymu daily settlement                         │
  │                payment_transaction.status: paid → settled      │
  │                Money lands in Lustia bank account              │
  │                Tenant earnings accrue (not yet disbursed)      │
  └───────────────────────────────────────────────────────────────┘
            │
            ▼
  ┌───────────────────────────────────────────────────────────────┐
  │ Weekly         Lustia ops trigger payout per tenant            │
  │                tenant_disbursement row created                 │
  │                Manual bank transfer Lustia → tenant            │
  │                payment_transaction.status: settled → disbursed │
  └───────────────────────────────────────────────────────────────┘
```

### 2.2 Provider abstraction

**`PaymentProvider` interface** declared in `service/interfaces.go` (consumer-owned per project convention):

```go
type PaymentProvider interface {
    // CreateQR initiates a transaction. Returns QRIS string + provider txn id.
    CreateQR(ctx context.Context, req CreateQRRequest) (CreateQRResponse, error)

    // VerifyWebhook validates signature + parses payload.
    VerifyWebhook(ctx context.Context, payload []byte, headers map[string]string) (PaymentNotification, error)

    // GetStatus polls provider for transaction state. Used as fallback if
    // webhook missed.
    GetStatus(ctx context.Context, providerReference string) (PaymentStatus, error)

    // ListSettlements fetches the daily settlement report (provider already
    // disbursed money to Lustia bank). Returns items eligible to mark
    // payment_transaction.status = settled.
    ListSettlements(ctx context.Context, date time.Time) ([]SettlementItem, error)
}
```

Adapters under `internal/helper/payment/`:
- `dummy.go` (build tag `dev`/`local` only — same gating as ADR 0011)
- `ipaymu.go` (STUB until keys arrive — interface-compliant, returns `errors.New("not yet implemented")`)
- `factory.go` reads `PAYMENT_PROVIDER=dummy|ipaymu` env, returns the right adapter, `log.Fatal` on misconfiguration

### 2.3 Schema additions (migration 000029)

Three new tables. All tenant-scoped where applicable, RLS-enforced.

#### `payment_transaction`
| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL FK | denormalised from booking for fast tenant queries |
| `booking_id` | UUID NOT NULL FK CASCADE | one booking → one txn (Phase 6) |
| `provider` | TEXT NOT NULL CHECK in (`dummy`,`ipaymu`,`midtrans`) | |
| `provider_reference` | TEXT NOT NULL | iPaymu trx id |
| `qr_string` | TEXT NULL | for re-display before paid |
| `qr_image_url` | TEXT NULL | provider-hosted optional |
| `qr_expires_at` | TIMESTAMPTZ NOT NULL | hard cap 15min, see §2.7 |
| `expected_amount_idr` | BIGINT NOT NULL | from booking.total_price_idr |
| `received_amount_idr` | BIGINT NULL | populated on webhook |
| `platform_fee_idr` | BIGINT NULL | computed at settlement; see §2.4 |
| `tenant_net_idr` | BIGINT NULL | received_amount - platform_fee |
| `status` | TEXT NOT NULL CHECK in (`awaiting`,`paid`,`settled`,`failed`,`expired`,`disbursed`,`voided`) | |
| `paid_at` | TIMESTAMPTZ NULL | |
| `settled_at` | TIMESTAMPTZ NULL | |
| `disbursed_at` | TIMESTAMPTZ NULL | |
| `settlement_batch_id` | UUID NULL FK | populated when settled |
| `disbursement_id` | UUID NULL FK | populated when disbursed |
| `raw_webhook` | JSONB NULL | last webhook payload for audit |
| `created_at`, `updated_at` | TIMESTAMPTZ | |

Indexes:
- `(tenant_id, status)` partial WHERE status IN (`paid`,`settled`) — for tenant balance queries
- `(provider_reference)` UNIQUE — webhook lookup
- `(booking_id)` UNIQUE — one booking → one txn invariant
- `(status, qr_expires_at)` partial WHERE status='awaiting' — for expiry sweep

RLS: standard tenant isolation. Public flow (`__public__` sentinel) has read access to a single transaction by `(booking_id, code)` lookup at service-layer (mirrors booking pattern).

#### `settlement_batch`
| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `provider` | TEXT NOT NULL | |
| `settled_at` | TIMESTAMPTZ NOT NULL | timestamp from provider report |
| `total_amount_idr` | BIGINT NOT NULL | sum of all txns in this batch |
| `transaction_count` | INT NOT NULL | |
| `raw_payload` | JSONB | provider's full settlement report |
| `created_at` | TIMESTAMPTZ | |
| `created_by` | UUID NULL FK → user(id) | platform admin who triggered reconciliation |

No tenant scope — platform-level table.

#### `tenant_disbursement`
| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `tenant_id` | UUID NOT NULL FK | |
| `period_start` | DATE NOT NULL | inclusive |
| `period_end` | DATE NOT NULL | inclusive |
| `gross_amount_idr` | BIGINT NOT NULL | sum(received_amount) of disbursed txns |
| `platform_fee_idr` | BIGINT NOT NULL | sum(platform_fee) |
| `net_amount_idr` | BIGINT NOT NULL | gross - fee |
| `transaction_count` | INT NOT NULL | |
| `status` | TEXT NOT NULL CHECK in (`pending`,`processing`,`transferred`,`failed`,`cancelled`) | |
| `bank_reference` | TEXT NULL | bank transfer ID once done |
| `notes` | TEXT NULL | platform admin notes |
| `transferred_at` | TIMESTAMPTZ NULL | |
| `transferred_by` | UUID NULL FK → user(id) | |
| `created_at`, `updated_at` | | |

Indexes:
- `(tenant_id, period_start)` — tenant history
- `(status)` partial WHERE status IN (`pending`,`processing`) — admin work queue

RLS: tenant_admin can read only own-tenant rows. Platform-admin via super_admin bypass.

### 2.4 Platform fee — 5% flat per transaction

Computed at **settlement time** (not at payment time):
```
platform_fee_idr = floor(received_amount_idr * 0.05)
tenant_net_idr   = received_amount_idr - platform_fee_idr
```

Owner decision (2026-04-30): **5% flat**. Rationale: Lustia's revenue from each completed booking. Per-tenant override possible in Phase 7+ via `tenant.platform_fee_pct` column.

The fee is computed at settlement, not at payment, so partial settlement (rare for QRIS) does not double-charge. Once `settled`, the `platform_fee_idr` and `tenant_net_idr` columns are immutable.

### 2.5 Payout cadence — weekly (default)

Lustia platform-admin runs payout once per week (default Senin pagi for last Senin–Minggu period). The `tenant_disbursement.period_start/period_end` columns capture the window.

Payout calculation rule:
```
gross  = SUM(received_amount_idr)
         FROM payment_transaction
         WHERE tenant_id = X
           AND status = 'settled'
           AND settlement_batch.settled_at BETWEEN period_start AND period_end
           AND disbursement_id IS NULL
fee    = SUM(platform_fee_idr) -- same WHERE
net    = gross - fee
```

Per-tenant cadence override (daily / monthly) deferred to Phase 7.

### 2.6 Manual disbursement (Phase 6)

Lustia platform-admin opens a payout, sees calculated `net_amount_idr`, manually transfers via banking app, then marks the disbursement `transferred` with optional `bank_reference`. Auto-disbursement via banking API (Flip / BRI Open Banking) is Phase 7+.

State machine for `tenant_disbursement.status`:
```
pending ──[admin starts processing]──▶ processing ──[admin marks transferred]──▶ transferred
        ──[admin cancels]──▶ cancelled
                                         │
                                         └──[bank reject]──▶ failed
                                                              │
                                                              └──[admin retry]──▶ processing
```

Once `transferred`, all related `payment_transaction` rows have `status` updated to `disbursed` and `disbursed_at` set, in the same DB transaction.

### 2.7 Webhook + polling endpoint pair

**Webhook** `POST /api/v1/public/payments/webhook`:
- iPaymu signature verification (HMAC-SHA256 with shared `IPAYMU_WEBHOOK_SECRET`)
- Idempotent — same `provider_reference` arriving twice = no-op
- Amount-mismatch guard (security H-4): if `received_amount < expected_amount`, audit log + 200 OK (don't credit)
- Updates `payment_transaction.status = paid` + `booking.status = paid` in single tx

**Polling** `GET /api/v1/public/bookings/:code/payment-status`:
- Returns `{status, paid_at?, qr_expires_at}`
- Rate limit: 12/min per IP (1 request per 5 sec sustained)
- Used by Flutter app between webhook arrival and customer screen refresh

**Dev-only short-circuit** `POST /api/v1/public/payments/dummy-trigger`:
- Build tag `//go:build dev || local`
- Body `{code}` triggers the webhook handler internally with synthetic payload
- For Flutter dev simulate button

### 2.8 Booking creation flow change

`POST /public/bookings` response shape change:

**Before (Phase 5 dummy):**
```json
{ "booking_id", "code", "snap_token", "redirect_url", ... }
```

**After (Phase 6):**
```json
{
  "booking_id", "code", "total_price_idr",
  "qr_string": "00020101...",
  "qr_image_url": "https://...",
  "qr_expires_at": "2026-04-30T16:30:00+07:00",
  "payment_reference": "ipaymu_trx_xxx"
}
```

The dummy `snap_token`/`redirect_url` fields removed. Frontend renders QR via `qr_flutter` from `qr_string`.

### 2.9 Settlement reconciliation (manual ops flow Phase 6)

Phase 6: platform-admin button "Tarik laporan iPaymu" triggers `provider.ListSettlements(yesterday)` → backend creates `settlement_batch` row + bulk-update matching `payment_transaction.status = settled`. Mismatches (iPaymu reports a txn that Lustia doesn't have, or vice versa) flagged as audit log warnings for ops investigation.

Auto-cron Phase 7+ (no cron infra in repo yet — manual button is acceptable for usaha kecil).

### 2.10 Refund / void

**Phase 6: no customer-initiated refund** (matches ADR 0014 §3.5).

Edge case ops cancellation post-paid: `tenant_disbursement` calculation excludes voided txns; `payment_transaction.status = voided` blocks settlement participation. Customer money stuck at iPaymu — manual refund via iPaymu support (out-of-band) per §2.10 is acceptable for Phase 6 (rare event).

Phase 7 will wire iPaymu void/refund API.

### 2.11 Permissions (migration 000030)

New permissions:
- `finance.read` — tenant_admin, branch_admin (own-tenant view)
- `finance.read_all` — super_admin (platform-wide)
- `disbursement.create` — super_admin only
- `disbursement.transfer` — super_admin only
- `settlement.reconcile` — super_admin only

UUID namespace `c0000000-0000-0000-0030-*` (verify-clean before write).

### 2.12 UI

**Tenant-admin (port 3002) — new "Keuangan" tab:**
- "Saldo Anda": breakdown card
  - Dalam proses (paid, awaiting settle): Rp X
  - Siap dicairkan (settled, awaiting disbursement): Rp Y  
  - Sudah dicairkan: Rp Z (link to history)
- "Riwayat Pencairan" — list `tenant_disbursement` rows with status badges
- "Riwayat Transaksi" — list `payment_transaction` rows with status badges + booking link
- Filter: date range, status

**Platform-admin (port 3001) — new "Settlement & Payout" section:**
- "Reconciliation" page: button "Tarik Laporan iPaymu Hari Ini" + history of settlement_batch
- "Tenant Payout" page: per-tenant cards with settled-but-not-disbursed amounts; click → review + create disbursement + manual transfer flow
- "Active Disbursements" list: rows in pending/processing — admin marks transferred when bank done

**Ops portal (port 3003):** no changes (ops doesn't touch finance).

**Customer-facing Flutter:** no changes beyond §2.7/§2.8. Customer never sees settlement or disbursement.

### 2.13 Non-goals (deferred)

- Auto-disbursement via banking API (Flip, BRI Open Banking)
- Customer-initiated refund flow
- Per-tenant payout cadence override
- Per-tenant platform fee override
- Settlement cron job (manual button works at usaha kecil scale)
- Multi-currency
- Booking partial-payment / deposit
- iPaymu void API integration
- Settlement reconciliation discrepancy auto-resolution
- Tax / PPN / faktur pajak generation
- Statement PDF export
- Real-time websocket payment status (polling is fine for QRIS at MVP scale)

## 3. Implementation order

1. ADR 0015 written (this doc).
2. db-designer → migration 000029 (payment lifecycle tables) + 000030 (permissions).
3. Parallel:
   - go-expert → backend (refactor MidtransClient → PaymentProvider, dummy/ipaymu adapters, polling endpoint, dev-trigger endpoint, payment service, settlement service, disbursement service, all new endpoints).
   - ui-ux-expert → DESIGN_SYSTEM.md spec for Keuangan + Payout pages (KEU-A/B and POU-A/B/C sections).
4. Parallel:
   - nextjs-expert → tenant-admin Keuangan + platform-admin Payout pages.
   - flutter-expert → PaymentScreen rework (QR display + countdown + polling + dev simulate).
5. Apply migrations, rebuild backend, restart, verify.
6. Commit + push.

## 4. Open questions

1. **Settlement timing variance.** iPaymu's "H+1" can be H+1 to H+3 depending on weekend/holiday. The reconciliation flow handles this naturally (we just receive batches when iPaymu sends them). Document expectation in ops runbook.
2. **Failed bank transfer in disbursement.** If Lustia transfers but bank rejects (wrong rekening), `tenant_disbursement.status = failed`. Admin re-edits + retries. Not blocking today.
3. **iPaymu account is perorangan vs PT.** iPaymu allows both. Owner can use KTP-only signup for MVP. NPWP optional for sole proprietorship.
4. **Settlement payload format.** iPaymu API for settlement listing not yet documented in our adapter — needs real account to inspect. Stub interface assumes `{transactions[]}` shape; adjust when real key arrives.
5. **Booking cancellation post-paid.** Currently blocked at ADR 0014 §3.5. Phase 7 may relax with refund flow. For now, ops cancel pre-paid only.
