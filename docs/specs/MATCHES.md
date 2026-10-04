# Matches — MA-01 đến MA-11

PO APPROVED 1.0 — Nguồn PRD E05–E10; D-03–D-06, D-12–D-18, D-25.
Mục tiêu: một suất hợp lệ, đúng lịch, đủ điều kiện và có thể vận hành/đối soát.
Actor: Player, Host của kèo, Admin có case. Tiền điều kiện công bố theo GL-02; hậu điều kiện mọi lượt/capacity/lịch nhất quán.
Không đặt hộ, waitlist, QR hoặc giải đấu trong MVP.

## Luồng theo spec

| Spec | Luồng chính | Ngoại lệ cần xử lý |
| --- | --- | --- |
| MA-01 Tạo/công bố | Nháp → sân đã xác nhận ngoài app → thời gian/format/level/capacity/phí/rules → kiểm quyền → công bố | Không đủ quyền/thiếu sân/thời gian quá khứ giữ nháp |
| MA-02 Khám phá | Nearby/vùng → lọc thời gian/skill/format/phí → card → detail | Không có GPS dùng vùng; không kết quả cho đổi bộ lọc |
| MA-03 Eligibility | Kèo mở/tương lai → tài khoản/hồ sơ → chặn → level/trust → lịch → suất | Mismatch skill có thể chuyển request nếu Host cho phép; không override trùng lịch/sức chứa |
| MA-04 Tham gia | Instant hoặc request → Host duyệt nếu cần → kiểm lại → miễn phí JOINED, có cọc HELD | Bị từ chối không giữ suất; trạng thái chờ phải hiển thị rõ |
| MA-05 Suất | Tạo hold → chờ báo tiền/xác nhận → chuyển hold thành confirmed hoặc hết hạn | Duyệt nhiều người cùng lúc phải xét suất tại thời điểm cam kết |
| MA-06 Host quản lý | Xem request/confirmed/hold → duyệt/từ chối/loại có lý do | Loại người có tiền phải phát sinh xét hoàn, không xóa giao dịch |
| MA-07 Hủy/thay đổi | Ghi người/lý do/time/policy → rút/hủy → xử lý suất/lịch/tiền/thông báo | Khi có HELD/JOINED áp hạn chế thay đổi D-15; không âm thầm đổi thỏa thuận |
| MA-08 Điểm danh | Host xác nhận có mặt/no-show trong cửa sổ → người chơi được biết → khiếu nại/sửa sai | Chưa check-in không tự no-show; sửa sau đóng qua case |
| MA-09 Vòng đời | Đến giờ bắt đầu → kết thúc → Host chốt attendance → hoàn thành | Sau end+24h tự hoàn thành; attendance chưa kết luận giữ UNKNOWN theo D-18 |
| MA-10 Đánh giá | Người đủ điều kiện chọn chất lượng 1–5/Host 1–5/tags/skill feedback → gửi | Không tự đánh giá bản thân/kèo hủy; NO_SHOW/UNKNOWN không được review |
| MA-11 Hoạt động | Upcoming/Hosted/History theo chủ tài khoản | Kèo đang hoàn tiền vẫn có thể ở History; không đồng nhất tiền với lịch hoạt động |

## Vòng đời nghiệp vụ đã duyệt và guard

| Đối tượng | Từ → đến | Trigger/guard |
| --- | --- | --- |
| Kèo | DRAFT → OPEN | Host đạt D-03; dữ liệu/sân hợp lệ |
| Kèo | OPEN ↔ FULL | Không còn suất có thể cấp ↔ suất được giải phóng trước cutoff |
| Kèo | OPEN/FULL → IN_PROGRESS | Khi đến giờ bắt đầu; cutoff thời gian chặn join dù trạng thái chưa cập nhật |
| Kèo | IN_PROGRESS → COMPLETED | Host kết thúc hợp lệ hoặc hệ thống kết thúc tại end+24h theo D-18 |
| Kèo | DRAFT/OPEN/FULL → CANCELLED | Host/Admin được phép, có lý do; tiền xử lý riêng |
| Kèo | IN_PROGRESS → CANCELLED | Chỉ Admin qua case; Host kết thúc sớm bằng COMPLETED và ghi lý do |
| Lượt | REQUESTED → APPROVED/REJECTED | Host quyết định; APPROVED chưa giữ suất nếu bước cấp hold thất bại |
| Lượt | APPROVED → AWAITING_PAYMENT/JOINED | Xét lại lịch/suất; cọc hay miễn phí |
| Lượt | AWAITING_PAYMENT → JOINED/EXPIRED/CANCELLED | Host xác nhận + hold hợp lệ / hết hạn / rút |
| Lượt | JOINED → CHECKED_IN/NO_SHOW/CANCELLED/REMOVED | Attendance hoặc hủy/loại theo quyền |
| Lượt | CHECKED_IN → COMPLETED | Kèo hoàn thành; NO_SHOW giữ kết quả riêng |
| Hold | HELD → CONSUMED/EXPIRED/RELEASED | Chuyển chính thức / hết hạn / hủy |

Đây là các outcome nghiệp vụ đã duyệt, không bắt buộc tên enum vật lý.
REQUESTED không giữ chỗ. AWAITING_PAYMENT kèm HELD giữ cả suất và khoảng thời gian theo D-13.
Một APPROVED cạnh tranh suất thất bại phải trả lý do; không tự thu tiền trước khi có hold.

## Quy tắc

- **MA-BR01 [CHỐT D-03/D-05]:** chỉ công bố khi quyền hiệu lực và Host xác nhận sân ngoài app.
- **MA-BR02 [CHỐT PRD E07]:** không nhận trùng lượt có hiệu lực; kiểm khoảng giao nhau existingStart < newEnd AND existingEnd > newStart; giờ liền kề không trùng.
- **MA-BR03 [CHỐT D-13]:** occupied = confirmed chưa rút/loại + HELD còn hạn; occupied <= capacity. CONSUMED không đếm thêm một hold.
- **MA-BR04 [CHỐT D-12]:** Host chỉ chiếm suất nếu khai có chơi, tạo lượt tương ứng; không đồng nhất Host với participant. Host chơi miễn thu tiền của mình, không tự đánh giá mình.
- **MA-BR05 [CHỐT D-13/D-16]:** xác nhận tiền chỉ chuyển JOINED nếu hold còn hiệu lực, kèo còn nhận, lịch/quyền còn hợp lệ; ngược lại Payments xử lý tiền muộn.
- **MA-BR06 [CHỐT D-15]:** không giảm capacity dưới occupied; khi có HELD/JOINED cấm đổi venue, start/end, format, fee/deposit; Host phải hủy và tạo kèo mới.
- **MA-BR07 [CHỐT PRD E09]:** Host xác nhận no-show, không tự gán do thiếu check-in.
- **MA-BR08 [CHỐT D-18]:** một CHECKED_IN/COMPLETED participant có một đánh giá/kèo/đích trong 7 ngày từ match `completedAt`; sửa qua case có lịch sử; không tự rate.
- **MA-BR09 [CHỐT ADR]:** retry join/hủy/điểm danh không tạo thêm kết quả; phiên bản sửa phải phân biệt với request gửi lặp.
- **MA-BR10 [CHỐT D-13/D-15]:** hủy hết hiệu lực suất/lịch ngay theo quyết định, không chờ hoàn tiền; nghĩa vụ tiền vẫn tồn tại.

## Bảng quyết định tham gia

| Điều kiện | Kết quả |
| --- | --- |
| Kèo quá giờ/đóng/hủy, khóa, bị chặn theo policy, trùng lịch | Không cấp suất; không yêu cầu chuyển tiền |
| Sai skill nhưng cho request | Chờ duyệt; không chiếm suất |
| Approval Required, chưa duyệt | REQUESTED; không cung cấp hướng dẫn thu tiền |
| Đã đủ điều kiện, miễn phí, còn suất | JOINED |
| Đã đủ điều kiện, cần cọc, còn suất | HELD + AWAITING_PAYMENT, hướng dẫn thanh toán riêng |
| Host xác nhận tiền, hold hết hạn | Không tự JOINED; Payments xử lý khoản nhận |
| Request lặp khi đã JOINED/HELD | Trả kết quả hiện hữu, không cấp suất thứ hai |

## Dữ liệu và phối hợp

Kèo cần title/description, Host, venue/court context, UTC start/end + IANA timezone,
format/style, level min/max, capacity, fee/deposit policy version, joinMode, rules,
venue confirmation actor/time, lifecycle và lịch sử thay đổi.
Lượt cần Player, kèo, eligibility snapshot/lý do, quyết định Host, thời điểm, hold,
attendance và revision. Hold có expiresAt/consumedAt/releasedReason.
Fee/deposit lấy từ cấu hình kèo theo D-14; cửa sổ hold theo D-13. Capacity nguyên dương; end > start; minLevel <= maxLevel.

Identity/Players cấp điều kiện; Venues cấp thông tin; Payments giữ nghĩa vụ/receipt;
Notifications và Communication nhận kết quả có phiên bản. Moderation quản lý case.
Mọi trường tiền riêng và giấy tờ không xuất trên match card. Quyền GL-02.

## Ví dụ và tiêu chí nghiệm thu

- **MA-AC01:** Given còn một suất; When A/B cùng join; Then tối đa một HELD/JOINED, người còn lại biết hết suất (BR02–03).
- **MA-AC02:** Given có lịch 18–20h; When join 19–21h; Then từ chối; join 20–22h không bị chặn vì overlap (BR02).
- **MA-AC03:** Given AWAITING_PAYMENT có HELD; When chuyển JOINED; Then occupied không tăng hai lần (BR03/05).
- **MA-AC04:** Given Host không chơi; When công bố capacity 8; Then có 8 suất Player; Host chơi thì còn 7 (BR04).
- **MA-AC05:** Given kèo có 6 occupied; When Host giảm capacity xuống 5; Then từ chối (BR06).
- **MA-AC06:** Given chưa check-in; When đến end+24h; Then attendance là UNKNOWN, không tự no-show và kèo vẫn có thể COMPLETED (BR07/D-18).
- **MA-AC07:** Given Player đã đánh giá; When gửi lặp; Then một đánh giá hiệu lực, không đánh giá chính mình (BR08–09).
- **MA-AC08:** Given lượt đã hủy đang chờ hoàn; When người khác join; Then suất có thể cấp theo policy, khoản hoàn vẫn tồn tại (BR10).
- **MA-AC09:** Given quyền thu hồi; When Host công bố; Then từ chối dù draft hợp lệ (BR01).
- **MA-AC10:** Given tiền đến sau hold hết hạn; When Host xác nhận tiền; Then không vượt capacity/trùng lịch để ép JOINED (BR05).

## Quyết định áp dụng

D-12–D-18 đóng Host/capacity, hold, hủy/refund, tiền muộn và attendance/review.
Tên bảng/enum có thể khác nhưng phải giữ độc lập Match, Participation, Hold và Payment lifecycle.
