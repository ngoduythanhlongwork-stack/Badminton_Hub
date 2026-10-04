# GL-01 — Thuật ngữ và ranh giới nghiệp vụ

Trạng thái: PO APPROVED 1.0. Nguồn: PRD E01–E14, D-01–D-25.
Mục tiêu: BA/UX/dev/QA dùng chung ý nghĩa trước khi thiết kế dữ liệu.
Tác nhân: tất cả actor; tiền điều kiện: đọc sổ quyết định; hậu điều kiện: thuật ngữ/owner được review.

## Từ điển

| Khái niệm | Định nghĩa nghiệp vụ | Owner |
| --- | --- | --- |
| Tài khoản | Danh tính đăng nhập duy nhất; email xác minh không chứng minh danh tính pháp lý | Identity |
| Hồ sơ người chơi | Dữ liệu cầu lông, vùng chơi, trình độ, sở thích; khác thông tin xác thực | Players |
| Quyền tổ chức | Khả năng được Admin cấp để công bố kèo sau điều kiện D-03 | Identity |
| Host của kèo | Tài khoản chịu trách nhiệm kèo cụ thể; quyền tổ chức không trao quyền lên kèo khác | Matches |
| Court Manager | Người có quyền quản lý địa điểm được giao; không tự có quyền Host | Identity cấp phạm vi, Venues áp dụng |
| Địa điểm | Cơ sở/sân cầu tại một địa chỉ, có thể có nhiều sân con | Venues |
| Sân con | Một sân vật lý trong địa điểm; tên/mã sân không phải lượt đặt | Venues |
| Kèo | Buổi chơi do một Host tổ chức, có khoảng thời gian, địa điểm, thể thức, sức chứa và điều kiện | Matches |
| Lượt tham gia | Quan hệ một Player với một kèo và vòng đời yêu cầu/tham gia | Matches |
| Suất | Một đơn vị sức chứa người chơi; số người trên sân cùng lúc khác số người của buổi chơi | Matches |
| Giữ chỗ | Quyền sử dụng tạm một suất, có thời hạn; chưa đồng nghĩa đã tham gia chính thức | Matches |
| Cọc | Khoản trả trước cố định trên một Player, được tính vào phí kèo theo D-14 | Payments |
| Phí tham gia | Tổng khoản Player phải trả cho kèo theo thỏa thuận; khác giá thuê sân | Payments giữ nghĩa vụ, Matches công bố đề nghị giá |
| Báo đã chuyển | Khai báo của Player về một lần chuyển tiền; chưa chứng minh Host nhận | Payments |
| Xác nhận đã nhận | Ghi nhận của Host về khoản thực nhận; app không xác minh ngân hàng | Payments |
| Nghĩa vụ hoàn | Khoản phải trả lại theo quyết định/policy; chưa có nghĩa tiền đã hoàn | Payments |
| No-show | Vắng mặt được Host xác nhận, không tự suy từ thiếu check-in | Matches |
| Trình độ | Ước lượng năng lực chơi, khác số trận và khác độ tin cậy | Players |
| Độ tin cậy | Tín hiệu hành vi tham gia/hủy/vắng; không phải kỹ năng cầu lông | Players |
| Điểm đề xuất | Mức phù hợp giữa một người và một kèo, có lý do; không phải rating của người | Recommendations |

## Quy tắc và ranh giới

- **GL01-BR01 [CHỐT D-04]:** không tạo tài khoản Host riêng; kiểm quyền trên từng kèo.
- **GL01-BR02 [CHỐT D-05]:** xác nhận sân từ Host là lời khai có thời điểm/người xác nhận, không phải bảo chứng booking.
- **GL01-BR03 [CHỐT D-06]:** tiền đi Player↔Host; nền tảng ghi nhận, không giữ tiền hoặc quyết toán.
- **GL01-BR04 [CHỐT PRD E07/E11]:** lưu tách lifecycle kèo, lượt tham gia, giữ chỗ, khoản thu/hoàn.
- **GL01-BR05 [CHỐT D-25]:** Matches quyết định đủ điều kiện/suất; Payments quyết định tiền đã ghi nhận; nhận tiền không tự vượt quyền của Matches.
- **GL01-BR06 [CHỐT ADR]:** mỗi module sở hữu dữ liệu/quyết định của mình; không đọc bảng nội bộ module khác.

## Luồng phối hợp và ngoại lệ

Identity cho biết trạng thái/quyền; Players cung cấp điều kiện hồ sơ; Venues cung cấp bối cảnh.
Matches xét tham gia, yêu cầu Payments ghi nghĩa vụ; Payments báo kết quả cho Matches kiểm tra lại.
Matches phát sự kiện điểm danh/hoàn thành; Players nhận tín hiệu; Notifications và Communication phản ứng theo quyền.
Moderation ra quyết định xử lý, owner thực thi và trả kết quả; Admin không sửa trực tiếp dữ liệu bỏ qua luật.

Ngoại lệ: thông tin sân sai → Venues sửa và Matches xử lý kèo liên quan; báo chuyển nhầm → Payments xử lý, không sửa capacity trước khi có quyết định hợp lệ.

## Dữ liệu và trạng thái

Từ điển trên là mô hình khái niệm, chưa xác định khóa/bảng.
Định nghĩa trạng thái nằm tại spec owner. Mọi thay đổi quan trọng cần actor, thời điểm, lý do và phiên bản quyết định.
Chi tiết dữ liệu: GL-06. Quyền: GL-02.

## Tiêu chí nghiệm thu

- **GL01-AC01:** Given A được duyệt tổ chức; When A mở kèo của B; Then A không có quyền Host của B (BR01).
- **GL01-AC02:** Given Host xác nhận đã đặt sân ngoài app; When xem kèo; Then không hiển thị “nền tảng bảo đảm đã đặt sân” (BR02).
- **GL01-AC03:** Given Player tải ảnh chuyển tiền; When chưa có Host xác nhận; Then không ghi nhận khoản đã nhận hoặc tự xác nhận tham gia (BR03–05).

## Quyết định áp dụng

D-12–D-17 chốt giữ suất, tiền và hoàn tiền. Thuật ngữ này không bắt buộc tên bảng/enum vật lý.
