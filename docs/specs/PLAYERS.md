# Players — PL-01 đến PL-05

PO APPROVED 1.0 — Nguồn PRD E02/E03/E10; D-01, D-09–D-10, D-18, D-22, D-25.
Mục tiêu: hồ sơ đủ để ghép kèo, tách kỹ năng và hành vi. Actor: Player, Host xem phần công khai, Admin xử lý sửa sai.
Tiền điều kiện: có tài khoản; hậu điều kiện: hồ sơ có mức hoàn tất/đủ tuổi và tín hiệu truy vết được.
Không xếp hạng giải đấu, gamification hay hồ sơ trẻ em trong pilot.

## Luồng và dữ liệu

| Spec | Luồng chính / dữ liệu | Ngoại lệ |
| --- | --- | --- |
| PL-01 Onboarding | Nhập tên, ngày sinh, kinh nghiệm, level, thể thức, style, khoảng thời gian, vùng chơi → lưu nháp → kiểm đủ dữ liệu/18 tuổi → hoàn tất | Chưa đủ 18 không mở quyền chơi/tổ chức; dữ liệu thiếu giữ nháp, quay lại tiếp tục |
| PL-02 Hồ sơ | Chủ sửa thông tin được phép → kiểm tra → công bố phần public | DOB theo D-10; đổi level không xóa lịch sử đánh giá |
| PL-03 Trình độ | Tự chọn level ban đầu → hiển thị tự đánh giá/chưa đủ dữ liệu → nhận tín hiệu sau trận | Không dùng độ tin cậy thay level; một ý kiến không là sự thật tuyệt đối |
| PL-04 Trust | Nhận kết quả attendance/cancel/feedback → nhận diện trùng → tạo tín hiệu → tổng hợp theo phiên bản | Sửa điểm danh phải thay/đảo tín hiệu cũ, không cộng hai lần |
| PL-05 Lịch sử | Xem lịch sử cá nhân và thống kê đã tính từ kết quả hợp lệ | Kèo hủy không tính hoàn thành; công khai theo D-09 |

Trường bắt buộc onboarding: tên hiển thị, DOB, kinh nghiệm, level, ít nhất một format/style/time-period, vùng chơi.
Avatar/gender tùy chọn (D-10). Vùng chơi là vùng lựa chọn, không bắt buộc vị trí GPS chính xác.
Danh mục level theo PRD: Beginner, Beginner+, Intermediate, Intermediate+, Advanced, Competitive.
Format: Singles/Doubles/Mixed; style: Casual/Social/Training/Competitive.
Kinh nghiệm: <3 tháng, 3–12 tháng, 1–3 năm, 3+ năm. Mốc bằng 3/12/36 tháng cần chuẩn hóa khi chốt dictionary.

## Trạng thái và quy tắc

Hồ sơ: NOT_STARTED → IN_PROGRESS → COMPLETE; đủ tuổi là điều kiện riêng được kiểm tra trước COMPLETE.
Trình độ: SELF_ASSESSED/ESTIMATED; độ tin cậy dữ liệu khác giá trị level.
Không có điểm mặc định “0 = xấu” cho người mới.

- **PL-BR01 [CHỐT D-01/D-10]:** dưới 18 không đủ điều kiện pilot; tính tuổi theo D-10.
- **PL-BR02 [CHỐT PRD E02]:** onboarding lưu được phần đã nhập; chỉ COMPLETE khi đủ điều kiện.
- **PL-BR03 [CHỐT PRD E10]:** phản hồi skill dùng As Expected/Stronger Than Profile/Lower Than Profile, không yêu cầu chấm điểm kỹ năng trực tiếp bằng số.
- **PL-BR04 [CHỐT D-22]:** completion, late cancel và confirmed no-show được tính theo công thức reliability D-22.
- **PL-BR05 [CHỐT D-18/D-22/D-25]:** nguồn tín hiệu gồm kèo/lượt/kết quả/phiên bản; sửa nguồn thay thế hiệu lực cũ để không đếm đôi.
- **PL-BR06 [CHỐT D-09]:** DOB/email/liên hệ riêng không đưa lên hồ sơ công khai; chỉ công khai social proof tối thiểu đã duyệt.

Phối hợp: Identity dùng trạng thái hồ sơ; Matches dùng eligibility và xuất attendance/feedback;
Moderation xử lý khiếu nại; Recommendations đọc tín hiệu qua hợp đồng, không tự sửa điểm.

## Tiêu chí nghiệm thu

- **PL-AC01:** Given DOB cho thấy chưa đủ 18; When hoàn tất; Then từ chối mở quyền tham gia và giải thích điều kiện (BR01).
- **PL-AC02:** Given onboarding nhập dở; When quay lại; Then còn dữ liệu đã lưu, không hiển thị COMPLETE (BR02).
- **PL-AC03:** Given newcomer; When xem profile; Then biết level là tự đánh giá, không gán no-show/uy tín xấu vì thiếu dữ liệu (BR03–04).
- **PL-AC04:** Given no-show đã tạo tín hiệu; When Admin sửa thành có mặt; Then lịch sử còn, tín hiệu tổng hợp phản ánh bản sửa đúng một lần (BR05).
- **PL-AC05:** Given người xem không phải chủ; When mở profile; Then không lộ DOB/email/bằng chứng tài chính (BR06).

## Quyết định áp dụng

D-09–D-10, D-18 và D-22 đã đóng privacy/DOB, correction và công thức skill/reliability.
Aggregate có thể được thiết kế nhưng phải tái tạo được từ source signal có version.
