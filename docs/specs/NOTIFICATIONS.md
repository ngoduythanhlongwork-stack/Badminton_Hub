# Notifications — NO-01 đến NO-05

PO APPROVED 1.0 — Nguồn PRD E12; D-10, D-18, D-20, D-24, D-25. Actor: người nhận, Host, vận hành.
Mục tiêu: thông báo đúng trạng thái đã được xác nhận. Tiền điều kiện: sự kiện owner hợp lệ; hậu điều kiện: nghĩa vụ thông báo có kết quả gửi.
Phạm vi in-app/email; push và quảng cáo không nằm đợt này.

## Luồng theo spec

| Spec | Luồng | Ngoại lệ |
| --- | --- | --- |
| NO-01 Sự kiện | Nhận event → xác định recipient/template/version → tạo nghĩa vụ | Không gửi cho người không đủ quyền hoặc bằng chứng nhạy cảm |
| NO-02 In-app | Tạo unread → người nhận mở → read | Đọc không ảnh hưởng trạng thái nghiệp vụ; đánh dấu lặp không tạo mới |
| NO-03 Email | Tạo pending → gửi → sent/failed → retry có giới hạn | Sent không là “đã đọc”; gửi lỗi không hủy join |
| NO-04 Nhắc | Lập lịch theo giờ kèo → kiểm trạng thái trước gửi → gửi/hủy | Đổi giờ phải hủy lịch cũ; kèo hủy/người rút không nhắc đi chơi |
| NO-05 Chống lặp | Khóa nghiệp vụ event+recipient+channel+purpose → kiểm đã xử lý | Event revision mới có thể là thông báo mới, retry event cũ không lặp |

## Danh mục sự kiện đã duyệt

| Sự kiện | Người nhận | Ý nghĩa nội dung |
| --- | --- | --- |
| Xác minh/reset | Chủ email | Link truy cập riêng; không chứa mật khẩu |
| Duyệt/từ chối/thu hồi Host | Người xin/giữ quyền | Kết quả và bước tiếp theo |
| Request tham gia | Host | Có yêu cầu chờ duyệt, chưa chắc chiếm suất |
| Duyệt/từ chối, giữ chỗ/hết hạn | Player liên quan | Trạng thái và hạn có hiệu lực |
| Player báo chuyển | Host nhận tiền | “Cần kiểm tra”, không “đã nhận tiền” |
| Host xác nhận tiền / không thấy | Player | Kết quả tiền; tham gia vẫn theo Matches |
| Join chính thức/full | Player/Host tương ứng | Suất đã xác nhận hoặc không còn suất |
| Hủy/đổi kèo/loại | Người bị ảnh hưởng | Thay đổi, quyền chọn, trạng thái tiền riêng |
| Hoàn: đến hạn/báo gửi/xác nhận/dispute | Bên cần hành động | Không gọi báo gửi là hoàn tất |
| Nhắc lịch | Participant xác nhận và Host | Thời gian/sân phiên bản mới |
| Attendance sửa/case có kết quả | Bên liên quan | Quyết định, đường xem chi tiết |
| Kèo hoàn thành/mời đánh giá | Người đủ điều kiện rate | Cửa sổ 7 ngày theo D-18 |

Phân loại kênh, opt-out reminder và retry áp dụng D-20.

## Quy tắc / trạng thái / dữ liệu

- **NO-BR01 [CHỐT PRD E12]:** kênh MVP là in-app và email; sự kiện đúng đối tượng.
- **NO-BR02 [CHỐT D-25]:** diễn đạt đúng nguồn/thực trạng, không nâng report chuyển/hoàn thành thực nhận.
- **NO-BR03 [CHỐT ADR]:** delivery failure không đảo business outcome; event lặp không tăng số thông báo nghĩa vụ.
- **NO-BR04 [CHỐT D-20]:** check trạng thái hiện hành trước reminder; schedule cũ được vô hiệu hóa khi đổi/hủy; nhắc tại 24h và 2h nếu mốc còn ở tương lai.
- **NO-BR05 [CHỐT guardrail]:** template không chứa credential/evidence/tài khoản nhận tiền; link yêu cầu quyền truy cập.
Trạng thái delivery: PENDING → SENT/FAILED → RETRY_PENDING → SENT/ABANDONED; in-app UNREAD/READ riêng.
Data: source event/version, recipient, channel, purpose, content version, scheduled time, send attempts/status/read time.
Email retry tối đa 3 lần với backoff khi sự kiện còn giá trị; không dùng vòng retry vô hạn.

## Tiêu chí nghiệm thu

- **NO-AC01:** Given Player chỉ báo chuyển; When email tới Host; Then nội dung yêu cầu kiểm tra, không xác nhận đã nhận (BR02).
- **NO-AC02:** Given join thành công; When email lỗi; Then join vẫn hiệu lực và trạng thái gửi ghi thất bại (BR03).
- **NO-AC03:** Given event phát lại; When xử lý; Then chỉ một nghĩa vụ/recipient/channel/purpose (BR03).
- **NO-AC04:** Given kèo đổi 19h sang 20h; When reminder cũ tới hạn; Then không nhắc giờ cũ (BR04).
- **NO-AC05:** Given email mời xem giao dịch; When chuyển tiếp người khác; Then người khác không truy cập được nội dung riêng (BR05).

## Quyết định áp dụng

D-10, D-18 và D-20 đóng token, cửa sổ review, reminder, retry và tùy chọn kênh.
