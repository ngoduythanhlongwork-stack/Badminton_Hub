# Venues — VE-01 đến VE-05

PO APPROVED 1.0 — Nguồn PRD E04/E05, D-05, D-09, D-11, D-15, D-25.
Mục tiêu: bối cảnh sân đáng hiểu để đánh giá kèo. Không booking, giữ sân, chống trùng lịch thuê hay thanh toán sân.
Actor: khách/Player xem, Host chọn, Court Manager/Admin quản lý.
Tiền điều kiện sửa: được cấp phạm vi; hậu điều kiện: thông tin có nguồn/ngày cập nhật và trạng thái công bố.

## Luồng theo spec

| Spec | Luồng chính | Ngoại lệ |
| --- | --- | --- |
| VE-01 Quản lý | Tạo địa điểm → thông tin/ảnh/sân con → review → công bố | Điều kiện công bố theo D-11; trùng địa điểm cần review không tạo tự do |
| VE-02 Phạm vi quản lý | Admin giao địa điểm → Court Manager chỉnh đúng phạm vi | Thu hồi quyền không xóa địa điểm hoặc đổi Host của các kèo |
| VE-03 Khám phá | Chọn vùng/khoảng cách/giá → danh sách → chi tiết | Thiếu GPS cho chọn vùng; không có kết quả hiển thị rõ |
| VE-04 Giá/khả dụng | Nhập giá tham khảo + đơn vị + nguồn + thời điểm; thông tin giờ mở nếu có | Không có lịch sân thực thì hiển thị “chưa có thông tin”, không suy là còn sân |
| VE-05 Gắn kèo | Host chọn địa điểm/sân con → khai báo đã tự đặt → lưu xác nhận với kèo | Venue chưa có gửi đề nghị bổ sung; sau khi có HELD/JOINED không đổi venue (D-15) |

## Trạng thái nghiệp vụ đã duyệt

DRAFT → PUBLISHED → HIDDEN/INACTIVE; Admin/Court Manager trong phạm vi thực hiện theo D-11.
INACTIVE không tự hủy các kèo tham chiếu; Matches nhận ảnh hưởng và yêu cầu Host/Admin xử lý.
Tên/địa chỉ đổi không được làm mất bối cảnh kèo lịch sử.

## Quy tắc

- **VE-BR01 [CHỐT D-05]:** “Host đã xác nhận có sân” chỉ là xác nhận từ Host, không tạo reservation.
- **VE-BR02 [CHỐT D-11]:** chỉ địa điểm PUBLISHED được gắn kèo mới; Host không tự cấp quyền sửa danh mục.
- **VE-BR03 [CHỐT D-05]:** giờ hoạt động/khả dụng tham khảo không cam kết có sân tại thời điểm đó.
- **VE-BR04 [CHỐT D-11/D-25]:** giá có đơn vị/nguồn/thời điểm; không chuyển giá sân thành phí mỗi người.
- **VE-BR05 [CHỐT D-04]:** Court Manager không tự duyệt người chơi hoặc nhận tiền kèo nếu không là Host.
- **VE-BR06 [CHỐT D-25]:** giữ bối cảnh địa điểm tại thời điểm công bố/thay đổi kèo để giải thích lịch sử.

## Dữ liệu và quyền

Tên/địa chỉ/vùng/toạ độ/time-zone/sân con: dữ liệu venue; địa chỉ và tọa độ phải cùng địa điểm.
Ảnh/tiện ích/giờ mở/giá: nguồn và cập nhật, field không có dữ liệu phải được phân biệt với giá trị 0.
Liên hệ quản lý riêng không mặc nhiên công khai. Không thu thông tin ngân hàng của sân để xử lý phí kèo.
Matches sở hữu lời xác nhận đã đặt sân và bản bối cảnh kèo; Venues sở hữu danh mục.
Khách xem public theo D-09; quản lý chỉ sửa phạm vi được giao.

## Tiêu chí nghiệm thu

- **VE-AC01:** Given Host tự xác nhận sân; When xem kèo; Then hiển thị nguồn Host, không thông điệp bảo đảm booking (BR01).
- **VE-AC02:** Given không có tồn sân; When tìm theo thời gian; Then không khẳng định có sân trống từ giờ mở cửa (BR03).
- **VE-AC03:** Given giá 100.000 VND/giờ; When kèo có phí riêng; Then hiển thị hai khái niệm riêng, không tự chia đầu người (BR04).
- **VE-AC04:** Given manager sân A; When sửa sân B hoặc duyệt người chơi kèo tại A; Then từ chối (BR05).
- **VE-AC05:** Given venue ngừng hoạt động; When cập nhật danh mục; Then kèo cũ vẫn truy vết bối cảnh và có luồng xử lý riêng (BR06).

## Quyết định áp dụng

D-11 chốt quyền/dữ liệu venue và hoãn rating/filter rating; D-09 chốt dữ liệu công khai;
D-15 cấm đổi venue khi đã có HELD/JOINED.
