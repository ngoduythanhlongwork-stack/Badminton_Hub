# GL-07 — Đo lường và yêu cầu chất lượng

PO APPROVED 1.0 — Nguồn PRD §7/§8, Blueprint §8; D-12, D-18, D-24, D-25.
Mục tiêu: đo được vòng đời thật, không đếm việc nhấn nút như thành công. Actor: PO, vận hành, QA.
Tiền điều kiện: sự kiện nguồn đã có outcome; hậu điều kiện: chỉ số có định nghĩa/version và mẫu số kiểm chứng được.

## Sự kiện và dữ liệu đã duyệt

OnboardingCompleted, SearchSubmitted, RecommendationExposed, MatchDetailViewed,
JoinStarted, JoinConfirmed, JoinRejected, TransferReported, ReceiptAcknowledged,
ParticipationCancelled, MatchCancelled, AttendanceConfirmed/Corrected,
MatchCompleted, ReviewSubmitted, RefundReported/Confirmed, CaseResolved.
Đây là tên nghiệp vụ, chưa là schema kỹ thuật. Mỗi event cần ID, source/version, occurredAt,
actor pseudonymous khi phù hợp, match/participation tham chiếu, reason và metric-definition version.
Không ghi JoinConfirmed từ TransferReported. Frontend retry/exposure nhiều lần phải có quy tắc phiên/count rõ.

## Định nghĩa chỉ số đã duyệt

| Chỉ số | Tử số / mẫu số | Cửa sổ và ngoại lệ |
| --- | --- | --- |
| Đăng ký | Số tài khoản đăng ký duy nhất | Tách verified/COMPLETE; không dùng tất cả làm “đã sẵn sàng chơi” |
| MAU | Unique Player có search/detail/join/check-in/review hoặc quản lý kèo trong tháng | Loại tài khoản test |
| Kèo hoàn thành/tháng | MatchCompleted duy nhất | Tháng theo timezone pilot; correction phải được phản ánh |
| Fill rate | Tổng confirmed ở thời điểm bắt đầu / tổng capacity snapshot của kèo thực diễn ra | Tỷ lệ có trọng số; không tính hold; Host chơi tính cả numerator/denominator; kèo hủy báo riêng |
| Search → detail | Unique search session có detail view / unique search session | Session hết hạn sau 30 phút không hoạt động |
| Detail → join | Unique cặp Player/kèo xem rồi JOINED / unique cặp Player/kèo có detail view | Trong 7 ngày sau view và trước giờ kèo; cohort đủ cửa sổ |
| Repeat 30 ngày | Player có lần chơi xác nhận thứ hai trong 30 ngày sau lần đầu / cohort Player chơi lần đầu đã quan sát đủ 30 ngày | Không tính chỉ gửi request lần nữa; không dùng cohort mới chưa đủ ngày |
| No-show rate | Lượt có no-show được xác nhận / lượt dự kiến chơi khi bắt đầu | Loại hủy/removed trước bắt đầu; có mặt+no-show+chưa giải quyết cần đối chiếu; sửa attendance tính lại |
| Chất lượng dữ liệu attendance | Lượt chưa kết luận / tổng lượt dự kiến | Báo riêng để không làm no-show giả thấp |

Các target 300–500 đăng ký, 100 MAU, 30–50 kèo/tháng, fill >60%, search→view >40%,
view→join >20%, repeat >30%, no-show <10% là giả thuyết pilot từ Blueprint, không bảo đảm kinh doanh.

## Yêu cầu chất lượng

Baseline PRD: API thường dưới 500ms, recommendation dưới 1s, availability pilot 99%,
WCAG 2.2 AA cho hành trình chính, HTTPS, bảo vệ dữ liệu và rate limit.
Dùng p95 ở tải pilot D-24: 50 concurrent users, 500 accounts, 10.000 matches lịch sử và
1.000 candidate còn hiệu lực; đo sau warm-up. Availability theo tháng, loại maintenance báo trước.
Mất mạng không hiển thị join/payment thành công nếu server chưa xác nhận. Hiển thị pending,
cho truy vấn lại kết quả; retry không tạo thêm nghĩa vụ.

## Quy tắc

- **GL07-BR01 [CHỐT PRD §8]:** instrument từ outcome thật; báo chuyển khác nhận tiền, giữ chỗ khác JOINED.
- **GL07-BR02 [CHỐT D-24]:** mỗi metric lưu định nghĩa/mẫu số/window/exclusion version; không đổi tên giữ số cũ âm thầm.
- **GL07-BR03 [CHỐT guardrail]:** retry/correction không nhân event nghiệp vụ hoặc đếm attendance sai; corrections truy về nguồn.
- **GL07-BR04 [CHỐT PRD §7]:** offline không giả thành công; bảo vệ thông tin và khả năng truy cập.
- **GL07-BR05 [CHỐT D-24]:** target latency dùng p95 ở dataset/concurrency pilot đã duyệt; báo rõ môi trường đo.

## Tiêu chí nghiệm thu

- **GL07-AC01:** Given Player báo chuyển nhưng chưa JOINED; When tính conversion; Then không tính join thành công (BR01).
- **GL07-AC02:** Given 10 lượt dự kiến, 8 có mặt, 1 no-show, 1 chưa giải quyết; When báo cáo; Then no-show 1/10 và unresolved 1/10 hiển thị riêng (BR02).
- **GL07-AC03:** Given no-show sửa thành có mặt; When tính lại; Then loại tín hiệu cũ, không nhân denominator (BR03).
- **GL07-AC04:** Given mất mạng sau gửi join; When UI chưa biết kết quả; Then pending và truy vấn lại, không xác nhận giả (BR04).
- **GL07-AC05:** Given môi trường chưa đạt dataset/concurrency D-24; When đo latency; Then không dùng kết quả để tuyên bố đạt target pilot (BR05).

## Quyết định áp dụng

D-12 và D-24 đóng cách tính Host participant, metric window/exclusion, privacy, retention, tải và availability.
