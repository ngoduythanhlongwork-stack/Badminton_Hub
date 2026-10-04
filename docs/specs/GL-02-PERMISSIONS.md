# GL-02 — Ma trận quyền và điều kiện thao tác

Trạng thái: PO APPROVED 1.0. Nguồn D-01–D-06, D-08–D-09, D-19, D-25.
Mục tiêu: quyền theo hành động + đối tượng + trạng thái. Không dùng “Admin làm mọi thứ”.
Tiền điều kiện: danh tính và phạm vi quyền được biết; hậu điều kiện: hành động được cho phép hoặc từ chối có lý do, không thay đổi dữ liệu khi bị từ chối.

## Actor

Khách; tài khoản chưa xác minh; Player đã xác minh/đủ 18/hoàn tất hồ sơ;
người có quyền tổ chức; Host của kèo; Court Manager được giao sân; Admin có quyền xử lý tương ứng.
Một người có thể giữ nhiều khả năng, nhưng vẫn bị giới hạn trạng thái/đối tượng.

## Ma trận quyền đã duyệt

| Hành động | Ai | Phạm vi và điều kiện |
| --- | --- | --- |
| Xem sân/kèo công khai | Khách và tài khoản | Chỉ nội dung được công bố, không thông tin chuyển tiền hay hồ sơ riêng |
| Xác minh/reset | Chủ email theo token | Token hợp lệ; không cấp quyền Host |
| Sửa hồ sơ | Chủ tài khoản | DOB sau onboarding chỉ sửa qua case theo D-10 |
| Gửi yêu cầu tham gia | Player đủ điều kiện | Kèo mở, tương lai, eligibility đạt |
| Xin quyền tổ chức | Player đủ điều kiện | Chưa có yêu cầu PENDING; hồ sơ theo D-08 |
| Duyệt/từ chối/thu hồi tổ chức | Admin có quyền | Có lý do, không tự duyệt hồ sơ của mình |
| Công bố kèo | Người được duyệt tổ chức | Sở hữu bản nháp, đã xác nhận sân, dữ liệu hợp lệ |
| Duyệt/loại/check-in | Host của kèo | Trạng thái cho phép; không bỏ qua sức chứa/trùng lịch |
| Xác nhận nhận tiền | Host nhận tiền của kèo | Đúng nghĩa vụ giao dịch; không xác nhận thay Host khác |
| Báo đã chuyển/xác nhận nhận hoàn | Player của khoản tiền | Đúng giao dịch thuộc mình |
| Sửa thông tin sân | Court Manager/Admin | Chỉ địa điểm được giao hoặc quyền quản trị riêng |
| Xem bằng chứng chuyển tiền | Player liên quan, Host nhận, Admin xử lý case | Không công khai; Admin cần mục đích xử lý |
| Gửi/đọc phòng kèo | Thành viên đủ điều kiện/Host | Theo CO-01–03; Court Manager không tự được vào |
| Đánh giá | Người tham gia hợp lệ | Kèo hoàn thành, phạm vi/cửa sổ theo MA-10 |
| Xử lý báo cáo | Admin có quyền case | Audit; nội dung chỉ trong phạm vi case |
| Hủy/điều chỉnh do quản trị | Admin được cấp quyền cụ thể | Lý do và case; đi qua workflow owner |

## Quy tắc

- **GL02-BR01 [CHỐT D-03]:** chỉ công bố khi email xác minh, hồ sơ hoàn tất, tuổi đủ, quyền tổ chức còn hiệu lực.
- **GL02-BR02 [CHỐT D-04]:** quyền tổ chức không cho quyền quản trị kèo khác.
- **GL02-BR03 [CHỐT guardrail]:** kiểm quyền ở mỗi thay đổi, không dựa vào nút đã hiển thị trước đó.
- **GL02-BR04 [CHỐT D-09]:** mặc định không cấp quyền nếu không có chính sách; hạn chế dữ liệu tiền/email/DOB theo nhu cầu.
- **GL02-BR05 [CHỐT D-19]:** khóa/thu hồi quyền không xóa nghĩa vụ tiền; xử lý qua kênh riêng, không mở toàn bộ quyền.
- **GL02-BR06 [CHỐT D-08/D-17]:** Admin không xác nhận tiền thay Host; có thể ghi kết quả tranh chấp với nguồn chứng cứ riêng.

## Luồng / trạng thái / dữ liệu

Yêu cầu → xác định actor → đối tượng → trạng thái → điều kiện → cho phép/từ chối → audit nếu nhạy cảm.
Quyền bị thu hồi giữa lúc người dùng mở màn hình và gửi thao tác: đánh giá lại, từ chối thao tác mới.
Dữ liệu cần: actor, action, object, phạm vi quyền, quyết định, lý do, thời điểm; không lưu mật khẩu/token vào audit.
Nguồn trạng thái tài khoản từ Identity, trạng thái kèo từ Matches, tiền từ Payments.

## Tiêu chí nghiệm thu

- **GL02-AC01:** Given đủ hồ sơ nhưng chưa duyệt Host; When công bố; Then từ chối, bản nháp còn nguyên (BR01).
- **GL02-AC02:** Given Court Manager quản lý sân của kèo; When duyệt người chơi; Then từ chối nếu không đồng thời là Host (BR02–04).
- **GL02-AC03:** Given quyền Host bị thu hồi sau khi mở trang; When công bố; Then kiểm quyền mới nhất và từ chối (BR03).
- **GL02-AC04:** Given khách; When xem kèo; Then không thấy email, ngày sinh, bằng chứng hoặc thông tin nhận tiền (BR04).
- **GL02-AC05:** Given tài khoản bị khóa còn khoản hoàn; When mở case; Then nghĩa vụ và chứng cứ vẫn tồn tại, không tự xóa (BR05).

## Quyết định áp dụng

D-08, D-09 và D-19 đóng duyệt/thu hồi, privacy và hạn chế tài khoản. Ma trận READY cho policy implementation.
