# Recommendations — RE-01 đến RE-05

PO APPROVED 1.0 — Nguồn PRD E14; D-22–D-25. Actor: Player, vận hành cấu hình được phân quyền.
Mục tiêu: đề xuất kèo phù hợp có giải thích; deterministic V1. RE-06 AI search beta nằm backlog, không đặc tả triển khai.
Tiền điều kiện: có candidate kèo và profile hoặc trạng thái thiếu dữ liệu; hậu điều kiện: danh sách lý do hoặc empty state, không thay quyền tham gia.

## Luồng theo spec

| Spec | Luồng | Ngoại lệ |
| --- | --- | --- |
| RE-01 Candidate | Lấy kèo public → loại không mở/tương lai/còn suất → lọc moderation/conflict | Không override eligibility của Matches |
| RE-02 Ranking | Chuẩn hóa skill/distance/time/style/trust → tính theo version D-23 → sort | Thiếu tín hiệu thì chuẩn hóa lại trọng số, không mặc định 0 |
| RE-03 Giải thích | Từ tín hiệu có nguồn → chọn reason trung thực → hiển thị | Không nói “gần bạn” nếu không có vị trí/vùng tính được; không % vô nghĩa |
| RE-04 Cold start | Dùng thông tin onboarding/vùng/time → danh sách phù hợp cơ bản | Không ép GPS; không có kết quả thì gợi ý đổi bộ lọc, không nới luật an toàn |
| RE-05 Version/đo | Lưu scoring version, candidate/filter outcome, reasons exposed → analytics | Config mới không viết lại kết quả lịch sử; retention theo D-24 |

## Quy tắc

- **RE-BR01 [CHỐT PRD E14]:** hard constraints trước weighted scoring; kèo đầy/hủy/quá khứ không được cứu bởi điểm cao.
- **RE-BR02 [CHỐT PRD E14]:** có reason thay vì chỉ phần trăm; reason phản ánh dữ liệu thật.
- **RE-BR03 [CHỐT D-23]:** cùng input + scoring version cho cùng thứ tự; hòa điểm theo giờ bắt đầu rồi mã ổn định.
- **RE-BR04 [CHỐT D-22/D-23]:** thiếu trust/GPS dùng fallback đã công bố; không đánh giá newcomer xấu vì thiếu lịch sử.
- **RE-BR05 [CHỐT PRD E07]:** khi Player join vẫn xét lại ở Matches; recommendation không bảo lưu suất.
- **RE-BR06 [CHỐT guardrail]:** chỉ owner Players thay rating/trust; Recommendations không truy cập dữ liệu riêng hoặc dùng AI quyết định eligibility.

## Giao diện nghiệp vụ, trạng thái và dữ liệu

Đầu vào: user/profile version, tiêu chí thời gian/vùng/format/phí, candidate snapshot, scoring version.
Đầu ra: danh sách kèo có thứ tự, reason codes/nội dung, thời điểm tính; hoặc EMPTY/INSUFFICIENT_CONTEXT.
Version cấu hình DRAFT → APPROVED → ACTIVE → RETIRED; PO duyệt trước khi ACTIVE.
Tín hiệu skill/trust là tham chiếu aggregate, không chứa raw evidence/payment/chat.
Trọng số V1: skill 35, distance 25, time preference 20, format/style 15, Host reliability 5.
Chuẩn hóa component, fallback, tie-break và giới hạn 20 kết quả theo D-23; endpoint/cấu trúc JSON là thiết kế kỹ thuật.

## Tiêu chí nghiệm thu

- **RE-AC01:** Given kèo điểm cao đã FULL; When đề xuất; Then bị loại trước xếp hạng (BR01).
- **RE-AC02:** Given không có dữ liệu khoảng cách; When tạo reason; Then không bịa số km (BR02).
- **RE-AC03:** Given input/version giống nhau; When tính lại; Then thứ tự ổn định theo tie-break đã duyệt (BR03).
- **RE-AC04:** Given newcomer không có trust; When ranking; Then áp fallback, không coi trust=0 xấu (BR04).
- **RE-AC05:** Given suất bị lấy sau khi xem đề xuất; When join; Then Matches từ chối hợp lệ, không cam kết suất từ card (BR05).

## Quyết định áp dụng

D-22–D-24 đóng skill/reliability, trọng số, fallback, tie-break và analytics. Spec READY cho V1 deterministic.
