# Communication — CO-01 đến CO-04

PO APPROVED 1.0 — Nguồn PRD E13; D-19, D-21, D-24, D-25. Actor: Host, participant đủ điều kiện, Admin case.
Mục tiêu: trao đổi trong phạm vi kèo; không messenger chung, nhóm xã hội hoặc inbox riêng.
Tiền điều kiện: có kèo và quan hệ thành viên hợp lệ; hậu điều kiện: message chỉ tới người còn quyền.

## Luồng theo spec

| Spec | Luồng chính | Ngoại lệ |
| --- | --- | --- |
| CO-01 Thành viên | Kèo công bố → phòng gắn kèo; Host có quyền; JOINED cấp quyền Player | REQUESTED/HELD chưa là thành viên; Court Manager không tự vào |
| CO-02 Tin nhắn | Kiểm quyền hiện tại → gửi văn bản → lưu actor/thời điểm → người được phép xem | Gửi lại cùng request không nhân message; không phát message chưa được chấp nhận |
| CO-03 Quyền sau thay đổi | Nhận rút/loại/hủy/hoàn thành → đổi quyền theo policy | Không dựa membership cache cũ để tiếp tục gửi |
| CO-04 Kiểm duyệt | Người có quyền report → case Admin → ẩn nếu có quyết định → giữ audit | Ẩn không xóa chứng cứ cần cho case; Host không xem phòng kèo khác |

## Ma trận quyền đã duyệt

| Quan hệ/trạng thái | Đọc | Gửi |
| --- | --- | --- |
| Host hợp lệ, kèo chưa đóng | Có | Có |
| Player JOINED/CHECKED_IN | Có | Có |
| REQUESTED/AWAITING_PAYMENT | Không | Không |
| CANCELLED/REMOVED của Player | Đọc lịch sử đến thời điểm mất quyền trong 30 ngày | Không |
| Participant của kèo CANCELLED | Đọc lịch sử 30 ngày | Không |
| Participant COMPLETED | Đọc 30 ngày | Tối đa 7 ngày sau kết thúc |
| Admin có case | Chỉ dữ liệu cần điều tra | Không giả danh thành viên |

Block không xóa message chung và không ẩn nội dung cần thiết của kèo đã cùng tham gia (D-19).
Membership là nguồn từ Matches; quyền hạn chế là nguồn từ Moderation/Identity.

## Quy tắc

- **CO-BR01 [CHỐT PRD E13]:** chỉ Host/participant xác nhận được tham gia phòng.
- **CO-BR02 [CHỐT D-21]:** kiểm quyền khi đọc/gửi sau thay đổi thành viên; rút/loại không còn gửi hoặc đọc message mới.
- **CO-BR03 [CHỐT guardrail]:** dữ liệu riêng được bảo vệ; Admin truy cập theo case và audit.
- **CO-BR04 [CHỐT D-25]:** retry không nhân message; thời điểm server là thứ tự nghiệp vụ, không tin đồng hồ client.
- **CO-BR05 [CHỐT D-21]:** chỉ văn bản UTF-8 đã trim 1–2.000 ký tự; không attachment/read-receipt/edit/delete tự do.

Dữ liệu: phòng thuộc kèo; member provenance; message ID nghiệp vụ, sender, body, created time, moderation state.
Thông tin tài khoản nhận tiền chính thức nằm Payments, không coi số tài khoản gửi trong chat là hướng dẫn thanh toán của hệ thống.

## Tiêu chí nghiệm thu

- **CO-AC01:** Given Player đang chờ duyệt/cọc; When vào phòng; Then từ chối dù xem được kèo public (BR01).
- **CO-AC02:** Given vừa bị loại; When gửi bằng trang đã mở; Then kiểm lại và từ chối (BR02).
- **CO-AC03:** Given Admin không có case; When đọc phòng riêng; Then không mặc nhiên được truy cập (BR03).
- **CO-AC04:** Given gửi thành công nhưng mất phản hồi; When retry cùng request; Then không nhân tin (BR04).
- **CO-AC05:** Given người gửi thông tin nhận tiền khác trong chat; When Player thanh toán; Then nguồn chính thức vẫn là hướng dẫn Payments có version (BR03).

## Quyết định áp dụng

D-19/D-21 đóng block, định dạng, giới hạn và quyền sau đóng; D-24 chốt retention 180 ngày cho dữ liệu message.
