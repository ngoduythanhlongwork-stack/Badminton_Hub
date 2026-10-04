# Payments — PA-01 đến PA-06

PO APPROVED 1.0 — Nguồn PRD E11; D-06, D-13–D-17, D-25.
Mục tiêu: ghi đúng nghĩa vụ, tiền Host xác nhận nhận và khoản phải hoàn; nền tảng không nắm giữ/chuyển tiền.
Actor: Player, Host nhận tiền, Admin xử lý tranh chấp. Không cổng thanh toán, callback, escrow, wallet hoặc settlement.
Tiền điều kiện yêu cầu thanh toán: Matches cấp lượt/hold hợp lệ; hậu điều kiện: nguồn xác nhận và tiền truy vết được, không tự cấp suất.

## Luồng theo spec

| Spec | Luồng chính | Ngoại lệ |
| --- | --- | --- |
| PA-01 Nghĩa vụ | Chụp phí/cọc/currency/policy đã chấp nhận → tạo nghĩa vụ theo lượt → hiển thị người nhận/hướng dẫn | Miễn phí không tạo receipt giả; thay phí không sửa hồi tố |
| PA-02 Báo chuyển | Player nhập số tiền/thời điểm/tham chiếu, bằng chứng nếu policy yêu cầu → SUBMITTED → Host kiểm nhận | Ảnh không đủ xác nhận; gửi lặp không thêm khoản thực nhận |
| PA-03 Xác nhận | Host ghi tiền thực nhận → đối chiếu đủ/thiếu/thừa/muộn → gửi kết quả Matches | Thiếu chưa đủ; thừa tách phần thừa; không nhận được ghi lý do, giữ report |
| PA-04 Xác định hoàn | Nhận hủy/loại/đổi từ Matches → dùng policy snapshot + thời điểm → tạo refund obligation | Không áp dụng policy mới hồi tố; case đặc biệt cần quyết định có lý do |
| PA-05 Thực hiện hoàn | Host chuyển ngoài app → báo đã hoàn/bằng chứng → Player xác nhận → đóng nghĩa vụ | Player phủ nhận/Host chưa hoàn → case; im lặng không tự hoàn tất |
| PA-06 Đối soát | Host/Player xem dòng tiền của mình; Admin xử lý case có evidence | Sửa xác nhận nhầm qua adjustment có lịch sử, không xóa receipt |

## Các vòng đời nghiệp vụ đã duyệt

| Đối tượng | Trạng thái / chuyển | Điều kiện |
| --- | --- | --- |
| Nghĩa vụ thu | OPEN → PARTIALLY_SATISFIED → SATISFIED hoặc CANCELLED | Theo tiền thực nhận đủ; hủy nghĩa vụ không xóa tiền đã nhận |
| Báo chuyển | SUBMITTED → ACKNOWLEDGED/NOT_FOUND/DISPUTED | Host xác nhận thực nhận/không thấy; Player có thể mở dispute |
| Khoản thực nhận | RECORDED → ADJUSTED nếu sai | Host/decision Admin có nguồn chứng cứ, revision; không ghi nhận hai lần |
| Nghĩa vụ hoàn | DUE/OVERDUE → REPORTED_SENT → CONFIRMED hoặc DISPUTED | Quá 48 giờ thành OVERDUE; Host báo gửi; Player xác nhận; Admin giải quyết khác nguồn |
| Hold/lượt | Xem Matches | Payments không sở hữu, không tự kéo dài hold |

Các outcome trên đã duyệt theo D-14/D-17; không tái dùng SUCCESS của cổng thanh toán để nói ngân hàng đã xác nhận.
Với xử lý case, ghi resolvedBy/source riêng, không giả actor Player hoặc Host.

## Quy tắc

- **PA-BR01 [CHỐT D-06]:** Player báo chuyển, Host xác nhận nhận; giao dịch ngoài app; hình ảnh/khai báo không tự thành thực nhận.
- **PA-BR02 [CHỐT PRD E11]:** tiền dùng số nguyên đơn vị nhỏ nhất + currency; tổng phí/cọc không là boolean IsPaid.
- **PA-BR03 [CHỐT D-14]:** 0 <= cọc <= phí; khoản cần trước khi JOINED là cọc đã chốt; phần còn lại có thể ghi nhận nhưng không đảo JOINED.
- **PA-BR04 [CHỐT ADR]:** lặp cùng report/xác nhận không nhân tiền. Hai lần chuyển thực sự khác nhau phải có receipt riêng; không dedupe chỉ dựa số tiền.
- **PA-BR05 [CHỐT D-16]:** tiền muộn vẫn phải ghi nhận nhưng không tự phục hồi suất; toàn bộ phần không còn nghĩa vụ hợp lệ được hoàn.
- **PA-BR06 [CHỐT PRD E11]:** policy và kết quả áp dụng được lưu; thay policy không sửa lịch sử.
- **PA-BR07 [CHỐT D-17]:** báo hoàn chưa là hoàn đã nhận; lưu người xác nhận, bằng chứng và nguồn quyết định; quá 48 giờ đánh dấu OVERDUE.
- **PA-BR08 [CHỐT D-25]:** tổng tiền hoàn xác nhận không vượt tổng thực nhận đã hiệu chỉnh cho cùng nghĩa vụ; adjustment không tạo số tiền âm không có giải trình.
- **PA-BR09 [CHỐT D-09]:** thông tin tài khoản nhận và evidence chỉ cho bên liên quan/Admin case; thay tài khoản nhận phải version hóa, report cũ gắn hướng dẫn cũ.

## Bảng quyết định tiền và suất

| Sự kiện | Kết quả tiền | Kết quả tham gia |
| --- | --- | --- |
| Player báo chuyển đủ | SUBMITTED, chưa coi là thực nhận | Giữ trạng thái chờ theo D-13 |
| Host nhận đủ, hold hợp lệ | Receipt đủ; nghĩa vụ đạt | Matches kiểm lại rồi JOINED |
| Host nhận thiếu | Receipt phần thực nhận | Chưa JOINED; yêu cầu bổ sung trong hạn |
| Host nhận thừa | Receipt toàn bộ, phần thừa thành nghĩa vụ hoàn | Xét JOINED theo deposit cần; không thêm suất |
| Host không thấy tiền | NOT_FOUND có lý do, giữ evidence | Không tự JOINED; case nếu bất đồng |
| Nhận sau hết hold/kèo hủy | Receipt muộn + nghĩa vụ xử lý | Không tự tái nhập |
| Host hủy kèo đã nhận | Hoàn đủ theo D-15 | Hủy và giải phóng suất độc lập |
| Host báo đã hoàn nhưng Player phủ nhận | DISPUTED | Không thay lịch sử tham gia để đóng case |

## Ví dụ (minh họa, không phải bảng giá đã duyệt)

Kèo phí 100.000, cọc 30.000 VND: chuyển 20.000 → ghi nhận 20.000, thiếu 10.000;
chuyển thêm 10.000 hợp lệ → đạt cọc; phần còn lại 70.000 có nghĩa vụ riêng.
Nếu nhận 40.000 cho yêu cầu cọc 30.000, tạo nghĩa vụ hoàn 10.000 thừa theo D-14; không tự coi mua thêm suất.
Chuyển 30.000 sau hết hold: ghi tiền đến muộn, không đẩy người khác khỏi suất.

## Dữ liệu, quyền và phối hợp

Nghĩa vụ: lượt/kèo, payer, payee, số tiền/currency, fee/policy snapshot, thời hạn.
Report: người báo, số tiền, thời gian khai báo, evidence tùy policy, mã tham chiếu.
Receipt: Host xác nhận, thời gian ghi nhận, số thực nhận, liên kết report, revision.
Refund: nguồn quyết định, khoản phải hoàn, hạn, báo thực hiện, xác nhận/đối soát và case.
Khóa nghiệp vụ/tham chiếu cần đủ đối chiếu nhưng chưa xác định bảng/ID vật lý.
Matches sở hữu lý do hủy và suất; Moderation xử lý tranh chấp; Notifications gửi trạng thái đúng ngữ nghĩa.

## Tiêu chí nghiệm thu

- **PA-AC01:** Given ảnh chuyển tiền; When tải lên; Then chưa SATISFIED nếu Host chưa nhận (BR01).
- **PA-AC02:** Given Host xác nhận lặp cùng receipt; When retry; Then tổng thực nhận không nhân đôi (BR04).
- **PA-AC03:** Given hai lần chuyển riêng cùng 30.000; When Host ghi từng lần; Then giữ hai receipt có nguồn riêng, không mất tiền do dedupe (BR04).
- **PA-AC04:** Given tiền đến sau hủy; When Host xác nhận; Then ghi tiền, tạo xử lý hoàn, không JOINED (BR05).
- **PA-AC05:** Given policy cũ tại lúc tham gia; When đổi policy; Then hủy áp đúng version được chấp nhận (BR06).
- **PA-AC06:** Given Host báo hoàn; When Player chưa xác nhận; Then hiển thị chờ xác nhận, không tự CONFIRMED (BR07).
- **PA-AC07:** Given nhận 30.000 đã hoàn 20.000; When xác nhận hoàn thêm 20.000; Then chặn/đưa đối soát, không vượt thực nhận (BR08).
- **PA-AC08:** Given chỉ nhận 20.000 của cọc 30.000; When đối chiếu; Then ghi thiếu, không xác nhận đủ (BR03).
- **PA-AC09:** Given người ngoài kèo; When yêu cầu evidence; Then từ chối (BR09).

## Quyết định áp dụng

D-13–D-17 đóng thời hạn hold, thiếu/thừa/phần còn lại, refund, tiền muộn và tranh chấp.
MVP không có partial refund; không được phục hồi participation chỉ vì đã nhận tiền.
