# Backend Implementation Plan — Matchmaking MVP

Trạng thái: **READY FOR EXECUTION — 2026-10-04**
Owner: Backend Team Lead
Nguồn: PRD, D-01–D-25, bộ spec 1.0 và ADR-001–ADR-017.

Execution status: **Slices 0–8 verified; R3 CLOSED — 2026-10-06**. The closed-loop MVP from
onboarding through play, review, correction-aware trust, deterministic recommendation and measurement
passes format, vet, unit, build, live PostgreSQL/Redis integration and HTTP E2E verification.

## 1. Mục tiêu và nguyên tắc thực thi

Xây backend Go modular monolith theo hành trình:

```text
onboard → discover → evaluate → join → pay → play → rate → recommend
```

Mỗi increment phải là một vertical slice có domain rule, persistence, API, authorization,
audit/observability và test. PostgreSQL là nguồn sự thật; Redis chỉ tăng tốc. Module không đọc
table/schema của module khác. Tên enum, bảng và endpoint có thể khác spec nhưng phải giữ nguyên
outcome, invariant, quyền và correction history đã duyệt.

Không đưa court booking, payment gateway, waitlist, push, QR, social feed, Club, Coach hoặc
Tournament vào kế hoạch này.

## 2. Hiện trạng và blocker đầu vào

Backend hiện có:

- Một Go module, một `cmd/api`, explicit composition và bounded graceful shutdown.
- PostgreSQL/Redis connection foundation, `/health`, `/ready`.
- Module-owned migration composition + `cmd/migrate`, stable HTTP errors/middleware,
  PostgreSQL transactional outbox/worker, isolated test database fixture và architecture guard.
- Business schemas `identity`, `players`, `venues`, `matches`, `payments`, `notifications`;
  `/api/v1` cho hành trình R2.
- Opaque session rotation, token-hash-only email workflow, scoped permissions và bootstrap Admin có audit.
- PostgreSQL-guarded free join, schedule/capacity/idempotency và E2E onboard → publish → join.

Môi trường R0 đã được xác nhận với Go 1.26.8, Docker Engine 25.0.3, PostgreSQL 17 và Redis 7.4.
PostgreSQL Compose dùng host port `55432` để không xung đột PostgreSQL host trên `5432`.

## 3. Technical decision gate — Iteration 0

Trước khi khóa implementation của slice tương ứng, tạo ADR cho các lựa chọn sau:

| ADR cần tạo | Khuyến nghị MVP | Gate |
| --- | --- | --- |
| Authentication | Built-in Identity; opaque access token 15 phút, rotating refresh token 30 ngày; chỉ lưu token hash | Trước Slice 1 |
| Password/email | Password hasher thay thế được; email sender interface + local capture adapter; production provider chọn trước pilot | Trước Slice 1 adapter |
| Migration | SQL migration theo owner module, một manifest/thứ tự deploy và một migrate command riêng | Trước table đầu tiên |
| API contract | `/api/v1`, JSON error code ổn định, correlation ID, pagination/filter convention, idempotency header cho command nhạy cảm | Trước public endpoint |
| Event/outbox/jobs | PostgreSQL outbox + worker trong cùng deployable; claim job bằng transaction; handler idempotent | Trước notification/expiry |
| Object storage | Adapter + object reference; chọn provider trước avatar/evidence upload production | Trước Slice 4 production |
| Map/geocoding | MVP cho Admin nhập tọa độ/vùng; provider geocoding không chặn venue catalog | Trước location automation |

Không cần message broker, microservice, distributed lock hoặc generic repository trong MVP.

## 4. Cấu trúc code mục tiêu

```text
src/backend/
├── cmd/
│   ├── api/                 # composition, HTTP server, workers
│   └── migrate/             # migration command
├── internal/
│   ├── httpapi/             # router, middleware, shared HTTP primitives
│   ├── platform/            # db transaction, clock, IDs, outbox, telemetry
│   └── modules/<module>/
│       ├── contracts.go     # cross-module queries/commands/events công khai
│       ├── module.go        # wiring/route registration công khai
│       ├── migrations/      # SQL do module sở hữu
│       └── internal/
│           ├── domain/
│           ├── application/
│           ├── postgres/
│           └── transport/
```

`platform` chỉ chứa technical primitives. Money, level, participation, payment state và permission
không được đẩy vào một shared-domain package.

## 5. Kế hoạch vertical slice

### Slice 0 — Implementation enablement

Outcome: có thể thêm business slice mà không phá module boundary.

- Chốt ADR ở mục 3 cần cho Slice 1.
- Thêm migration command, schema-per-module convention và migration integration test.
- Thêm HTTP middleware: request/correlation ID, panic recovery, bounded body, auth context,
  structured access log và stable error response.
- Thêm clock/ID abstraction cho domain test; transaction helper không chứa repository dùng chung.
- Thêm PostgreSQL outbox skeleton, worker lifecycle và graceful shutdown.
- Thêm architecture test cấm module import private code hoặc SQL schema của module khác.
- Tạo test fixture/database isolation convention cho integration test.

Exit gate: migration lên database rỗng thành công; migrate lại không đổi kết quả; handler mẫu chạy
qua transaction/outbox; architecture test bắt được dependency bị cấm.

### Slice 1 — Identity, session và Player onboarding

Outcome: người dùng đăng ký, xác minh email, đăng nhập và hoàn tất hồ sơ 18+ có thể tiếp tục hành trình.

Phạm vi:

- Identity: register, verify/resend, login, refresh rotation, logout, forgot/reset password,
  session revocation, account state và rate limit D-10.
- Players: onboarding draft/resume/complete, age eligibility, profile/preferences, public projection.
- Identity permission policies; organizer application/approval/revocation D-08 là sub-slice 1C.
- Notification outbox cho verification/reset; local capture adapter trong development/test.

Persistence owner:

- `identity`: account, credential, verification/reset token, session family, permission assignment,
  organizer application và security audit.
- `players`: profile draft/completion, preference và public player projection.

Test bắt buộc: token hết hạn/dùng lại, email canonicalization, password/session limits, refresh reuse,
reset revocation, DOB 29/2, onboarding resume, under-18 denial, permission scope và Admin conflict.

Exit gate: ID-AC01–06, PL-AC01–03/05 và GL02-AC01/03/04 đạt; credential/token không xuất hiện trong log.

### Slice 2 — Venue catalog và Match publishing

Outcome: organizer được duyệt có thể công bố một kèo tại venue hợp lệ.

- Venues: scoped Court Manager/Admin assignment, draft/publish/hide, public list/detail.
- Matches: draft/create/update/publish; Host chơi hoặc không chơi; snapshot venue/timezone;
  validation time/level/capacity/fee/deposit/rules.
- Policy kiểm `verified + COMPLETE + 18+ + organizer permission` tại thời điểm publish.
- Không có court inventory/reservation; Host attestation được audit rõ nguồn.

Test bắt buộc: manager vượt scope, venue chưa publish, quyền organizer bị thu hồi giữa request,
Host participant chiếm đúng một suất, UTC/IANA timezone và không sửa material field khi đã có participant.

Exit gate: VE-AC01–05, MA-AC04/09 và GL02-AC01–02 đạt.

### Slice 3 — Discovery, detail và free join

Outcome: primary journey chạy end-to-end không cần payment.

- Match search/list/detail với time, level, distance/area, format và cost filter.
- Eligibility orchestration qua explicit contracts: account/profile, block, skill/reliability,
  schedule conflict và capacity.
- Instant free join và Approval Required cho kèo miễn phí.
- PostgreSQL transaction/constraint bảo vệ một participation hiệu lực, overlap và capacity.
- Idempotency cho join/approve/reject/remove; public card không hứa còn suất.

Test bắt buộc: race nhiều request vào suất cuối, retry cùng idempotency key, cùng Player join lặp,
interval overlap/liền kề, request không giữ suất, approval mất suất và unauthorized Host.

Exit gate: MA-AC01–03/10, GL03-AC02 và free-flow E2E đạt dưới race test thực với PostgreSQL.
Đây là mốc demo nội bộ đầu tiên.

### Slice 4 — Paid hold và transfer acknowledgement

Outcome: kèo có cọc giữ suất đúng hạn nhưng không nhầm báo chuyển với tiền đã nhận.

- Matches: `HELD/AWAITING_PAYMENT`, 30 phút báo chuyển, extension tối đa 2 giờ và expiry job.
- Payments: fee obligation, nhiều transfer report/receipt thật, Host acknowledge/not-found,
  thiếu/thừa tiền và phần còn lại tại sân.
- Cross-module command/event: Payments không tự chuyển `JOINED`; Matches re-check hold, match,
  schedule và capacity trước confirmation.
- Payment instruction snapshot và privacy D-09; evidence là object reference, không log raw data.

Test bắt buộc: report không tự join, duplicate acknowledgement, hai transfer cùng amount,
partial deposit, overpayment, expiry/report race, Host acknowledge sau expiry và Redis outage.

Exit gate: PA-AC01–03/08–09, MA-AC03/10 và E2E-01 đạt.

### Slice 5 — Cancellation, refund và transactional notification

Outcome: hủy giải phóng suất độc lập với tiền, mọi refund truy vết được.

- Player cancel với mốc 6 giờ; Host cancel/remove hoàn đủ; không partial refund.
- Cấm material change sau `HELD/JOINED`; hủy rồi tạo kèo mới.
- Late money tạo refund obligation, không phục hồi participation.
- Refund `DUE/OVERDUE/REPORTED_SENT/CONFIRMED/DISPUTED`, deadline 48 giờ.
- In-app notification + email delivery worker, dedupe, 3 retry và reminder scheduling D-20.

Test bắt buộc: cancel/refund race, policy snapshot, late acknowledgement, refund vượt receipt,
event redelivery, email failure không rollback business transaction và reminder cũ bị vô hiệu hóa.

Exit gate: PA-AC04–07, MA-AC05/08, NO-AC01–05 và E2E-02 đạt.

### Slice 6 — Attendance, completion, review và reliability

Outcome: một kèo đã chơi tạo trust signal đúng và sửa sai được.

- Attendance từ start-30 phút đến end+24 giờ; không tự suy no-show.
- Completion worker; attendance chưa kết luận thành `UNKNOWN`, không bị phạt.
- Review 7 ngày từ `completedAt`; chỉ CHECKED_IN/COMPLETED, không self-review.
- Skill feedback/confidence và reliability cửa sổ 20 participation theo D-22.
- Admin correction trong 30 ngày, revision event và aggregate recomputation idempotent.

Test bắt buộc: completion khi Host im lặng, UNKNOWN không phạt, no-show không review,
review retry, correction thay signal cũ và reliability threshold boundary.

Exit gate: MA-AC06–07, PL-AC03–04, MO-AC04 và E2E-03 đạt.

### Slice 7 — Match room và Moderation tối thiểu

Outcome: participant trao đổi trong kèo và có đường xử lý abuse/payment dispute.

- Room membership lấy từ Matches; text 1–2.000 ký tự; idempotent send.
- Quyền sau cancelled/removed/completed, read cutoff và retention D-21/D-24.
- Report user/match/message, block/unblock, case lifecycle, assignee conflict, decision/action audit.
- RESTRICTED/SUSPENDED policy và read-only access cho case/nghĩa vụ tiền.

Test bắt buộc: HELD không vào room, stale page sau remove không gửi được, không đọc message mới,
Admin không case không đọc room, block không xóa nghĩa vụ và conflicting Admin không xử lý case.

Exit gate: CO-AC01–05, MO-AC01–05 và E2E-05 đạt.

### Slice 8 — Recommendation V1 và measurement

Outcome: Player nhận top 20 kèo deterministic, có lý do và funnel đo outcome thật.

- Candidate hard filters từ Matches contracts/read model; không query table module khác.
- Scoring version D-23: skill 35, distance 25, time 20, format/style 15, Host reliability 5;
  missing component được renormalize; tie-break ổn định.
- Reason codes chỉ từ dữ liệu có nguồn; không hiển thị phần trăm vô nghĩa.
- Analytics outbox/events, metric definition version và retention D-24.
- Query/read model/index tuning cho 1.000 active candidates; PostGIS chỉ thêm khi distance query được triển khai.

Test bắt buộc: hard filter luôn thắng score, missing GPS/trust fallback, newcomer neutral,
same input/version same order, correction không đếm đôi và p95 trên pilot dataset.

Exit gate: RE-AC01–05, GL07-AC01–05 và performance target đạt trong môi trường được ghi nhận.

## 6. Release gates

| Mốc | Bao gồm | Điều kiện ra mốc |
| --- | --- | --- |
| R0 Foundation ready | Slice 0 | ADR/migration/test harness/module boundary đạt |
| R1 Free matchmaking demo | Slice 1–3 | Onboard → publish → discover → free join; race test xanh |
| R2 Paid pilot core | Slice 4–5 | Hold/payment/cancel/refund/notification đối soát được |
| R3 Closed-loop MVP | Slice 6–8 | Play → rate → reliability → recommendation + measurement |

Không bắt đầu slice kế tiếp nếu migration, permission hoặc concurrency invariant của slice trước còn đỏ.
Có thể làm transport/UI contract song song sau khi application contract của slice đã ổn định; không làm
song song hai thay đổi cùng ownership table/state machine.

## 7. Test strategy và Definition of Done

Mỗi BR phải có unit/domain test hoặc integration test; mỗi AC phải truy được tới test case.

- Unit: domain transition, policy, scoring, clock boundary, permission.
- Repository integration: SQL constraint, transaction, migration và correction history.
- HTTP: authentication, authorization, validation, error code và idempotency response.
- Concurrency: join suất cuối, hold expiry/acknowledge, cancel/refund, worker redelivery.
- Architecture: import direction và không cross-schema SQL.
- E2E backend: năm scenario trong `TRACEABILITY.md`.

Definition of Done cho mỗi slice:

1. Migration + rollback/recovery note; schema do đúng module sở hữu.
2. API contract và stable machine-readable error code được ghi tài liệu.
3. Authorization policy, audit và PII/log review hoàn tất.
4. Unit/integration/concurrency test liên quan xanh; AC traceability được cập nhật.
5. `go fmt ./...`, `go vet ./...`, `go test ./...`, `go build ./...` xanh.
6. Không thêm dependency/runtime service nếu chưa có lý do và ADR phù hợp.

## 8. Rủi ro cần theo dõi

| Rủi ro | Cách kiểm soát |
| --- | --- |
| Toolchain local/CI lệch phiên bản | Pin Go 1.26.x trong CI và chạy cùng baseline trước merge |
| State machine payment/join bị trộn | Aggregate và schema owner riêng; cross-module command/event; E2E-01/02 |
| Race capacity/expiry/cancel | PostgreSQL transaction + constraint/locking; integration race test |
| Background job chạy lặp | Outbox/job idempotency key, claim lease và replay test |
| Auth/email vendor khóa thiết kế | Port/adapter; business token/session policy nằm trong Identity |
| PII/evidence lộ qua log | Chỉ lưu object ref, allowlist log fields, security test/review |
| Recommendation đọc chéo bảng | Contract/read model owner-controlled; architecture test |
| Scope phình thành booking/social | Review PRD/out-of-scope ở đầu mỗi slice |

## 9. Trạng thái bàn giao sau R3

Slices 0–8 đã được implement và R3 đã đóng. Bước tiếp theo là pilot hardening: nối production email,
chọn object storage/map provider, quan sát hiệu năng trên đúng dataset D-24 và tích hợp web theo API hiện có.
Không đưa Phase 2 vào backend nếu PRD chưa được PO sửa có chủ đích.
