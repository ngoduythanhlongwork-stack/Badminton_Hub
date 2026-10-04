# GL-03 — Hành trình đầu cuối và phối hợp liên module

Trạng thái: PO APPROVED 1.0. Nguồn D-01–D-25.
Mục tiêu: khép kín vòng khám phá → tham gia → chơi → đánh giá → đề xuất.
Actor: Player, Host, Admin, Court Manager. Không có cổng thanh toán hoặc hệ thống booking trong hành trình MVP.

## Luồng chính

| Hành trình | Tiền điều kiện | Các bước | Hậu điều kiện |
| --- | --- | --- | --- |
| Người chơi mới | Có email, đủ 18 | Đăng ký → xác minh email → hoàn tất hồ sơ → khám phá | Có quyền yêu cầu tham gia, chưa có quyền công bố |
| Xin quyền tổ chức | Player đủ điều kiện | Nộp yêu cầu → Admin review → chấp thuận/từ chối có lý do | Quyền tổ chức độc lập với kèo |
| Công bố kèo | Quyền tổ chức hiệu lực | Host đặt sân ngoài app → chọn sân → thông tin/phí/điều kiện → xác nhận có sân → công bố | Kèo có thể được tìm thấy |
| Kèo miễn phí | Kèo mở, đủ điều kiện | Yêu cầu → duyệt nếu cần → xét lại suất/lịch → xác nhận | Lượt tham gia chính thức, không tạo tiền giả bằng 0 |
| Kèo có cọc | Eligibility và suất đạt | Duyệt nếu cần → giữ chỗ → hiển thị hướng dẫn → Player báo chuyển → Host xác nhận → kiểm tra lại giữ chỗ → xác nhận tham gia | Tiền và tham gia có kết quả riêng |
| Chơi và đánh giá | Lượt hợp lệ | Nhắc lịch → Host điểm danh → hoàn thành → người đủ điều kiện đánh giá | Tín hiệu cho Players/Recommendations |
| Hủy/hoàn | Có lượt/tiền liên quan | Ghi lý do → quyết định hủy và giải phóng suất → tính nghĩa vụ hoàn → Host thực hiện → xác nhận hoặc tranh chấp | Hủy có thể xong trong khi hoàn tiền còn mở |

## Quy tắc

- **GL03-BR01 [CHỐT D-02/D-03]:** hoàn tất onboarding không tự cấp quyền tổ chức.
- **GL03-BR02 [CHỐT PRD E07]:** kiểm lại eligibility/sức chứa tại thời điểm xác nhận, không chỉ lúc xem kèo.
- **GL03-BR03 [CHỐT D-13/D-16]:** nhận tiền sau hết giữ chỗ không tự tạo suất; đưa sang xử lý tiền muộn.
- **GL03-BR04 [CHỐT D-06]:** người báo chuyển là Player, người xác nhận nhận là Host; không có webhook ngân hàng trong MVP.
- **GL03-BR05 [CHỐT ADR]:** retry cùng thao tác không tạo thêm lượt/ghi nhận tiền/thông báo.
- **GL03-BR06 [CHỐT D-20/D-25]:** notification/email lỗi không đảo kết quả nghiệp vụ; dùng sự kiện đã được owner xác nhận.

## Ngoại lệ bắt buộc

| Tình huống | Xử lý/phân công |
| --- | --- |
| Hai người tranh suất cuối | Matches chỉ cho một giữ chỗ/xác nhận hợp lệ; người còn lại nhận lý do hết suất |
| Host không xác nhận tiền | Hết thời hạn D-13 → không coi là đã nhận; hold hết, report còn để đối soát |
| Tiền đến muộn hoặc kèo đã hủy | Payments ghi khoản thực nhận; Matches không tái mở kèo/lượt; xử lý hoàn theo D-16 |
| Sai sân/đổi giờ | Nếu đã có HELD/JOINED, Host hủy và tạo kèo mới; hoàn theo D-15 |
| Khiếu nại no-show | Moderation xử lý; Matches sửa có lịch sử; Players tính lại tín hiệu |
| Sự kiện nhận lại nhiều lần | Consumer nhận diện kết quả đã xử lý; không đếm đôi |

## Trạng thái, dữ liệu và quyền

Dùng bốn vòng đời độc lập ở Matches/Payments, không dùng một cờ “đã thanh toán”.
Hành trình liên kết account, match, participation, hold, obligation, receipt/refund và case bằng tham chiếu nghiệp vụ.
Quyền theo GL-02; dữ liệu chi tiết GL-06; không thiết kế API wire format ở đây.

## Tiêu chí nghiệm thu

- **GL03-AC01:** Given Player đã onboarding; When chưa duyệt tổ chức; Then vẫn chỉ được xin quyền, không công bố (BR01).
- **GL03-AC02:** Given suất cuối; When hai lượt hợp lệ xác nhận đồng thời; Then chỉ một lượt giữ suất, sức chứa không vượt (BR02).
- **GL03-AC03:** Given tiền được Host xác nhận sau khi suất cấp người khác; When xử lý kết quả; Then giữ lịch sử tiền và không vượt sức chứa, mở xử lý tiền muộn (BR03–04).
- **GL03-AC04:** Given xác nhận tham gia thành công; When email thất bại hoặc sự kiện lặp; Then lượt không bị hủy/nhân đôi (BR05–06).

## Quyết định áp dụng

D-12–D-21 đóng các nhánh có tiền và vận hành. Luồng liên module này là baseline kiểm thử E2E.
