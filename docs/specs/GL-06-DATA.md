# GL-06 — Dữ liệu nghiệp vụ và khả năng truy vết

PO APPROVED 1.0 — Nguồn PRD §7/§8; D-09–D-24, D-25 và các spec owner.
Mục tiêu: đầu vào logic cho ERD sau review; không phải thiết kế bảng/khóa/index.
Actor: BA, PO, QA, developer; tiền điều kiện: hiểu glossary; hậu điều kiện: mỗi nhóm dữ liệu có owner, mục đích, quyền và lịch sử.

## Danh mục dữ liệu

| Nhóm / owner | Dữ liệu tối thiểu cần phân tích | Ràng buộc / quyền |
| --- | --- | --- |
| Account / Identity | email, verification, account state, credential reference, created/updated | Email duy nhất theo D-10; credential không đọc lại; DOB không copy vào Identity |
| Organizer application / Identity | applicant, contact riêng, reason, decision, reviewer, decided time | Một yêu cầu PENDING; không tự duyệt; review history |
| Permission assignment / Identity | người, capability, scope, effective/revoked time, source decision | Khác sở hữu kèo; nguồn cấp/thu hồi truy vết |
| Player / Players | tên, DOB, avatar/gender tùy chọn, level/experience, format/style/time/area | Điều kiện COMPLETE và 18+; DOB riêng, giới hạn sửa D-10 |
| Trust/skill signal / Players | source attendance/feedback, subject, type, effective version, confidence | Không đếm trùng; sửa nguồn đổi hiệu lực; công thức D-22 |
| Venue/court / Venues | tên, địa chỉ/toạ độ/timezone, sân con, giờ/tiện ích/ảnh, publication | Nguồn và thời điểm cập nhật; giá có unit, không phải fee kèo |
| Match / Matches | Host, bối cảnh sân, xác nhận sân, UTC start/end, timezone, format/style/level range, capacity, fee policy reference, rules, lifecycle | end > start; capacity nguyên dương; lịch sử thay đổi lớn |
| Participation / Matches | player, match, request/decision, eligibility outcome, attendance version | Một lượt hiệu lực cho player/kèo theo D-12; lý do từ chối có nguồn |
| Hold / Matches | participation, start/expiry, state, consume/release time/reason | Không đếm hold consumed thêm lần nữa; deadline D-13 |
| Fee obligation / Payments | payer, payee, participation, amount/currency, purpose, policy/price snapshot | Số nguyên tiền; cọc <= fee theo D-14; không sửa hồi tố |
| Transfer report / Payments | reporter, amount, claimed time, reference/evidence, status | Báo chuyển không là receipt; privacy hai bên + Admin case |
| Receipt/adjustment / Payments | Host acknowledgement, amount, linked report, time, source, revision | Retry không cộng trùng; hai chuyển thật có thể cùng amount |
| Refund / Payments | source cancellation/decision, policy version, due amount, sent report, confirmation/dispute | Không vượt thực nhận sau điều chỉnh; nghĩa vụ độc lập participation |
| Review / Matches | author, match, target, quality/host stars, tags/skill feedback, revision | Người đủ điều kiện; không self-review; cửa sổ D-18 |
| Case / Moderation | parties, reason, evidence ref, status, decision, assignee, actions/results | Chỉ đúng phạm vi; không xóa chứng cứ khi ẩn public |
| Room/message / Communication | match, membership provenance, sender, content, created, moderation state | Quyền từ participation; retention D-21/D-24 |
| Notification / Notifications | source event/version, recipient, channel, purpose, schedule, delivery/read | Không chứa dữ liệu nhạy cảm không cần thiết |
| Recommendation / Recommendations | input references, scoring version, result IDs/reasons, exposure | Không sao chép raw chat/evidence; retention D-24 |

## Quy tắc

- **GL06-BR01 [CHỐT ADR]:** mỗi nhóm một owner; tham chiếu liên module không cho phép sửa dữ liệu của nhau.
- **GL06-BR02 [CHỐT ADR]:** timestamp nghiệp vụ UTC + IANA timezone cho lịch; DOB là ngày lịch, không biến thành thời điểm UTC.
- **GL06-BR03 [CHỐT PRD E11]:** số tiền + currency + policy snapshot; không chỉ IsPaid.
- **GL06-BR04 [CHỐT D-25]:** quyết định nhạy cảm có actor/time/reason/source/version, giữ dấu vết correction.
- **GL06-BR05 [CHỐT guardrail]:** mật khẩu/token/raw payment evidence/chat riêng/vị trí chính xác không đưa vào log hoặc analytics.
- **GL06-BR06 [CHỐT D-21/D-24]:** áp retention/xóa/ẩn danh theo loại dữ liệu; không mặc định lưu vô hạn.

## Luồng kiểm soát thay đổi và trạng thái

BA thêm/sửa định nghĩa → xác định owner/privacy/validation → gắn BR/AC → review PO → cập nhật phiên bản → thiết kế ERD sau gate.
Dữ liệu sửa sai: giữ bản hiệu lực và lịch sử; không dùng “đã xóa” để mất nghĩa vụ tài chính.
ERD phải thể hiện một suất/tài khoản/kèo, Host participation tùy chọn và nhiều transfer receipt thật theo D-12–D-14.

## Tiêu chí nghiệm thu

- **GL06-AC01:** Given cùng khái niệm xuất hiện hai module; When review; Then chỉ một owner quyết định, bên kia dùng tham chiếu (BR01).
- **GL06-AC02:** Given lịch sân có timezone; When chuyển đổi hiển thị; Then cùng thời điểm, DOB không lệch ngày do UTC (BR02).
- **GL06-AC03:** Given hủy với policy cũ; When audit; Then tìm được giá/policy/actor đã áp (BR03–04).
- **GL06-AC04:** Given export analytics; When kiểm mẫu; Then không có credential/evidence/raw chat (BR05).
- **GL06-AC05:** Given một loại dữ liệu đến hạn retention; When job xử lý; Then xóa/ẩn danh đúng policy mà không làm mất nghĩa vụ tiền/case còn hiệu lực (BR06).

## Quyết định áp dụng

D-09–D-17 và D-21–D-24 đóng privacy, identity, cardinality, tiền và retention. Gate dữ liệu đã đạt.
