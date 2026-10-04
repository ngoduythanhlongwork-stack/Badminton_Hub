# Moderation — MO-01 đến MO-06

PO APPROVED 1.0 — Nguồn PRD §6, E08–E11; D-06, D-17–D-19, D-21, D-24, D-25.
Mục tiêu: tiếp nhận và giải quyết vi phạm/tranh chấp có lịch sử. Actor: người báo, bên bị báo, Admin có quyền.
Tiền điều kiện: đối tượng báo cáo xác định được; hậu điều kiện: case có quyết định hoặc yêu cầu bổ sung, owner thực thi có kết quả.
Không tự xác minh giao dịch ngân hàng; Admin không được gán rằng mình là Host nhận tiền.

## Luồng theo spec

| Spec | Luồng / dữ liệu | Ngoại lệ |
| --- | --- | --- |
| MO-01 Báo cáo | Chọn user/match/message → reason + mô tả/evidence → nộp → mã case | Trùng cùng sự việc liên kết case, không cộng số tố cáo thành phán quyết tự động |
| MO-02 Chặn | Player chặn/bỏ chặn người khác → thay điều kiện tương tác mới | Lượt/tiền đã tồn tại không tự hủy; phòng chung theo D-19 |
| MO-03 Xử lý | Admin tiếp nhận → kiểm phạm vi/xung đột lợi ích → thu thập → quyết định → owner thực thi → thông báo | Thiếu chứng cứ chờ bổ sung; không cam kết SLA public |
| MO-04 Hạn chế | Ghi biện pháp, phạm vi, thời hạn, lý do → Identity/Matches/Communication thực thi | Thu hồi Host không tự xóa kèo; phải có tác vụ xử lý người/tiền đang liên quan |
| MO-05 Tranh chấp | Attendance/rating/cancel/payment → đối chiếu chứng cứ → quyết định → owner điều chỉnh | Không kết luận bằng một ảnh; tiền thực chuyển do hai bên cung cấp thông tin |
| MO-06 Audit | Lưu quyết định, actor, trước/sau, lý do, evidence reference, phiên bản | Sửa sai bằng quyết định mới; không sửa lặng lịch sử |

## Trạng thái nghiệp vụ đã duyệt

SUBMITTED → TRIAGED → IN_REVIEW ↔ WAITING_INFORMATION → DECIDED → RESOLVED.
DECIDED chưa RESOLVED nếu owner chưa thực thi xong; REOPENED → IN_REVIEW khi có chứng cứ mới.
Mỗi bên được kháng nghị một lần trong 7 ngày; pilot không công bố SLA xử lý.

## Quy tắc

- **MO-BR01 [CHỐT PRD §6]:** report/block có lịch sử; reason gồm Spam/Fraud/Harassment/Fake Skill/No-show/Other.
- **MO-BR02 [CHỐT D-19]:** chặn ảnh hưởng tương tác mới theo cặp người, không giải phóng suất hoặc xóa tiền đã có.
- **MO-BR03 [CHỐT D-19]:** Admin không xử lý case có mình là bên liên quan; chuyển người khác.
- **MO-BR04 [CHỐT guardrail]:** owner module thực thi thay đổi qua hợp đồng/quy tắc; không thao tác trực tiếp vượt invariants.
- **MO-BR05 [CHỐT D-19]:** khóa không làm biến mất nghĩa vụ hoàn; bên bị hạn chế có kênh read-only theo dõi case riêng.
- **MO-BR06 [CHỐT D-24/D-25]:** case chỉ đóng khi kết quả nghiệp vụ được đối chiếu; gỡ nội dung không xóa chứng cứ riêng trong retention.

Phối hợp: Identity cấp hạn chế; Matches sửa attendance/hủy; Players sửa tín hiệu; Payments điều chỉnh đối soát;
Communication ẩn nội dung; Notifications thông báo phù hợp privacy.
Dữ liệu case/evidence chỉ các bên được phép và Admin phụ trách xem; không đưa chứng cứ của người khác nguyên bản ra public.

## Tiêu chí nghiệm thu

- **MO-AC01:** Given hai report cùng sự việc; When triage; Then liên kết để review, không tự khóa vì số lượng (BR01).
- **MO-AC02:** Given đã chuyển tiền; When bị chặn; Then vẫn còn giao dịch/khả năng mở case (BR02/05).
- **MO-AC03:** Given Admin là Host bị khiếu nại; When xử lý; Then chuyển case cho người không liên quan (BR03).
- **MO-AC04:** Given quyết định sửa no-show; When owner thực thi; Then attendance có revision và trust được sửa đúng nguồn, case mới RESOLVED khi thành công (BR04/06).
- **MO-AC05:** Given nội dung bị ẩn; When xem audit; Then người đủ quyền vẫn truy vết quyết định/chứng cứ, public không xem được (BR06).

## Quyết định áp dụng

D-17–D-19, D-21 và D-24 đóng tranh chấp, correction, block/lock, appeal và retention.
