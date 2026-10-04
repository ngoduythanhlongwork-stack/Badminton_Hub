# Identity — ID-01 đến ID-05

PO APPROVED 1.0 — Nguồn PRD E01; D-01–D-04, D-08–D-10, D-19, D-25.
Phạm vi: danh tính, phiên, quyền và yêu cầu tổ chức. Không xác minh danh tính pháp lý/KYC, không social sign-in trong baseline.
Actor: khách, chủ tài khoản, Admin. Tiền điều kiện: email truy cập được; hậu điều kiện: tài khoản/quyền có trạng thái rõ, không tự cấp quyền ngoài quy trình.

## Luồng theo spec

| Spec | Luồng chính | Ngoại lệ / kết quả |
| --- | --- | --- |
| ID-01 Đăng ký/xác minh | Email + mật khẩu → tạo tài khoản chưa xác minh → gửi link → mở link hợp lệ → xác minh | Email trùng không tạo tài khoản thứ hai; token sai/hết hạn không xác minh; gửi lại theo D-10 |
| ID-02 Phiên | Đăng nhập → kiểm tra thông tin/trạng thái → tạo phiên → refresh hoặc logout | Sai thông tin trả thông báo không tiết lộ email tồn tại; khóa tài khoản áp D-19 |
| ID-03 Khôi phục | Yêu cầu reset → phản hồi chung → chủ email dùng link → đổi mật khẩu → thu hồi phiên cũ | Link dùng rồi/hết hạn bị từ chối; không tự xác minh tài khoản chỉ vì reset |
| ID-04 Quyền tổ chức/phạm vi sân | Player đủ điều kiện gửi hồ sơ → Admin review → duyệt/từ chối có lý do → cấp khả năng tổ chức | Không tự duyệt; duyệt lại cùng yêu cầu không nhân đôi quyền; Court Manager được cấp theo địa điểm |
| ID-05 Hạn chế/khóa | Quyết định Admin/case → ghi lý do/phạm vi/hiệu lực → ngăn hành động mới → xử lý nghĩa vụ tồn | Không tự hủy kèo/tiền; không xóa lịch sử |

## Trạng thái nghiệp vụ đã duyệt

| Đối tượng | Chuyển trạng thái | Actor/guard |
| --- | --- | --- |
| Tài khoản | UNVERIFIED → ACTIVE | Chủ email dùng token hợp lệ; ACTIVE chưa nghĩa onboarding hoàn tất |
| Tài khoản | ACTIVE → RESTRICTED/SUSPENDED → ACTIVE | Admin có case/quyền và lý do theo D-19 |
| Yêu cầu tổ chức | NONE → PENDING → APPROVED/REJECTED | Player đủ điều kiện nộp; Admin quyết định |
| Quyền tổ chức | APPROVED → REVOKED | Admin; kèo đang mở xử lý theo D-08 |
| Yêu cầu tổ chức | REJECTED/REVOKED → PENDING | Cho nộp lại khi không còn yêu cầu PENDING |
| Phiên | ACTIVE → EXPIRED/REVOKED | Hết hạn/logout/reset/khóa theo D-10/D-19 |

## Quy tắc

- **ID-BR01 [CHỐT D-02]:** xác minh email là điều kiện tham gia/công bố; dùng email và mật khẩu.
- **ID-BR02 [CHỐT D-03]:** quyền tổ chức cần hồ sơ hoàn tất, đủ tuổi và Admin duyệt.
- **ID-BR03 [CHỐT D-10]:** một tài khoản theo email đã trim và so khớp không phân biệt hoa/thường; không tự xóa dấu chấm/+tag.
- **ID-BR04 [CHỐT D-10]:** link xác minh/reset dùng một lần, có hạn; retry link đã dùng không gây thêm side effect.
- **ID-BR05 [CHỐT D-08/D-19]:** thay đổi quyền có actor/lý do/hiệu lực; không chuyển sở hữu kèo hoặc quyền thu tiền ngầm.
- **ID-BR06 [CHỐT guardrail]:** trả quyết định quyền theo hành động/phạm vi; không coi Admin là quyền vô hạn.

## Dữ liệu nghiệp vụ và quyền

| Dữ liệu | Bắt buộc / kiểm tra | Quyền xem |
| --- | --- | --- |
| Email, trạng thái xác minh | Bắt buộc, chuẩn hóa theo D-10 | Chủ tài khoản; Admin đúng nhiệm vụ |
| Mật khẩu/credential | Bắt buộc khi đăng ký; chính sách D-10 | Không hiển thị lại cho bất kỳ actor |
| Phiên | Chủ, thời điểm tạo/hết hạn/thu hồi | Chủ và vận hành được phân quyền |
| Yêu cầu Host | Người nộp, lý do, liên hệ, trạng thái, lịch sử review | Người nộp + Admin; không công khai |
| Cấp quyền sân | Người được cấp, địa điểm, thời hạn/thu hồi nếu có | Admin + người được cấp |
| DOB/onboarding | Tham chiếu kết quả đủ tuổi/hoàn tất từ Players | Identity không tạo hồ sơ người chơi thứ hai |

Phối hợp: Players trả eligibility; Notifications gửi email; Moderation quyết định case;
Matches nhận thay đổi quyền để ngăn công bố mới và xử lý kèo tồn theo chính sách.

## Tiêu chí nghiệm thu

- **ID-AC01:** Given token xác minh hết hạn; When mở link; Then không xác minh, có đường gửi lại, không lộ token (BR01/04).
- **ID-AC02:** Given chưa hoàn tất hồ sơ; When xin/công bố kèo; Then chưa được công bố dù email đã xác minh (BR02).
- **ID-AC03:** Given email đã có tài khoản; When đăng ký lại; Then không tạo tài khoản trùng (BR03).
- **ID-AC04:** Given cùng link reset dùng hai lần; When lần hai gửi; Then không đổi mật khẩu lần hai (BR04).
- **ID-AC05:** Given Host bị thu hồi còn kèo có tiền; When thu hồi; Then dừng công bố mới, bảo toàn khoản tiền/kèo và tạo xử lý vận hành (BR05).
- **ID-AC06:** Given Admin không có phạm vi case; When xem credential/bằng chứng; Then từ chối truy cập ngoài quyền (BR06).

## Quyết định áp dụng

D-08–D-10 và D-19 đã đóng Q-01–Q-03/Q-12. Lựa chọn built-in hay external identity provider
vẫn là ADR kỹ thuật; giải pháp được chọn phải giữ nguyên token/session outcome và policy ở đây.
