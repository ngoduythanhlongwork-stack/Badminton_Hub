# Bộ đặc tả nghiệp vụ Badminton Hub MVP

Phiên bản: **1.0 — 2026-10-04**. Độc giả: Product Owner (PO), BA, UX, developer, QA.
**Trạng thái: PO APPROVED — sẵn sàng làm đầu vào ERD/API và triển khai backend theo vertical slice.**
D-01–D-25 là baseline bắt buộc. Q-01–Q-17 đã được đóng trong [sổ quyết định](DECISIONS.md).

## Cách đọc và phê duyệt

- [CHỐT]: yêu cầu đã được PO duyệt; mã D chỉ nguồn quyết định.
- Tham chiếu Q còn xuất hiện chỉ để truy vết câu hỏi review đã đóng, không phải blocker.
- Khi có mâu thuẫn: quyết định đã duyệt mới nhất phải được cập nhật đồng thời vào PRD và spec.
- PO duyệt chính sách; BA cập nhật phiên bản/ngày/ảnh hưởng; QA đối chiếu tiêu chí nghiệm thu.
- BR/AC không gắn mã D riêng được duyệt theo D-25. Chi tiết vật lý (bảng, enum, endpoint) do thiết kế kỹ thuật quyết định nhưng không được đổi outcome/invariant.

## Thứ tự và tài liệu

| Đợt | Spec | Đầu ra / gate |
| --- | --- | --- |
| 1 | [GL-01 Thuật ngữ](GL-01-GLOSSARY.md), [GL-02 Quyền](GL-02-PERMISSIONS.md), [GL-03 Hành trình](GL-03-JOURNEYS.md) | Một nghĩa cho một thuật ngữ; một owner cho một quyết định; áp dụng D-08–D-11 |
| 2 | [Identity](IDENTITY.md), [Players](PLAYERS.md), [Venues](VENUES.md) | Điều kiện đủ để chơi/tổ chức, dữ liệu và quyền xem |
| 3 | [Matches](MATCHES.md), [Payments](PAYMENTS.md) | Suất, thời hạn, tiền, hủy theo D-12–D-17 |
| 4 | Matches MA-08–11, Players PL-04–05, [Moderation](MODERATION.md), [Notifications](NOTIFICATIONS.md), [Communication](COMMUNICATION.md) | Vận hành, sửa sai, truy vết theo D-18–D-21 |
| 5 | [Recommendations](RECOMMENDATIONS.md), [GL-06 Dữ liệu](GL-06-DATA.md), [GL-07 Đo lường](GL-07-MEASUREMENT.md), [Truy vết](TRACEABILITY.md) | Scoring/data/measurement theo D-22–D-24 |

Mỗi file module gộp các spec con có mã ID/PL/VE/MA/PA/NO/CO/RE/MO.
Mã BR dùng cho quy tắc; AC cho tiêu chí Given–When–Then. Các bảng trạng thái là mô hình nghiệp vụ đã duyệt, không bắt buộc tên enum/database schema.

## Ranh giới đợt phân tích

Không viết source, migration, API schema hoặc ERD trong đợt này.
Không thêm booking trong app, cổng thanh toán, escrow/ví, AI search beta, waitlist,
QR check-in, push, Club, Coach Marketplace, Tournament hay social feed.
Lịch triển khai kỹ thuật cũ không quyết định thứ tự phân tích BA.

## Gate bàn giao sang ERD — ĐẠT

1. Q-01–Q-17 ảnh hưởng thực thể, quan hệ, lịch sử và trạng thái đã được PO quyết định.
2. Quyền của từng thao tác có phạm vi đối tượng; owner và nguồn dữ liệu xác định.
3. Các chuyển trạng thái có trigger, actor, guard và hậu điều kiện.
4. Các kịch bản tranh suất, trùng lịch, nhận tiền muộn, hủy và sửa điểm danh đã review.
5. Từ điển dữ liệu là đầu vào logic; thiết kế vật lý phải giữ module ownership theo ADR.

Agent BE phải triển khai từng lát dọc nhỏ và test business rule tương ứng. Không triển khai toàn bộ module
chỉ từ state table, không biến tên trạng thái nghiệp vụ thành enum/database schema nếu chưa có thiết kế kỹ thuật.

## Thứ tự handoff khuyến nghị cho backend

1. Identity + onboarding Player; chốt ADR authentication/email delivery trước phần adapter.
2. Venue catalog tối thiểu + tạo/công bố kèo.
3. Discovery/detail + free instant join, gồm idempotency, schedule conflict và race suất cuối.
4. Approval join + paid hold + transfer report/Host acknowledgement.
5. Cancellation/refund/late money + notification bắt buộc.
6. Attendance/completion/review + reliability signal/correction.
7. Match room + moderation tối thiểu.
8. Recommendation V1 + analytics outcome.

Mỗi lát phải có API, domain rule, persistence và test; chưa đi tiếp nếu invariant concurrency/permission của lát trước chưa đạt.
