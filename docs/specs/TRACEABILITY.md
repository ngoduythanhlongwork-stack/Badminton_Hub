# Truy vết PRD → spec → quy tắc → nghiệm thu

PO APPROVED 1.0 — 2026-10-04. Đây là kiểm phủ yêu cầu, không phải báo cáo test phần mềm đã chạy.
Q-01–Q-17 đã đóng bằng D-08–D-24; toàn bộ AC có hiệu lực nghiệm thu theo D-25.

## Bản đồ bao phủ

| Nguồn | Spec / tài liệu | Bộ quy tắc | Bộ nghiệm thu |
| --- | --- | --- | --- |
| E01–E14 / D-04–06 | [GL-01](GL-01-GLOSSARY.md) | GL01-BR01–06 | GL01-AC01–03 |
| E01, E05, E08 / D-03 | [GL-02](GL-02-PERMISSIONS.md) | GL02-BR01–06 | GL02-AC01–05 |
| E01–E14 / D-02–06 | [GL-03](GL-03-JOURNEYS.md) | GL03-BR01–06 | GL03-AC01–04 |
| E01 / D-01–04 | [ID-01–05](IDENTITY.md) | ID-BR01–06 | ID-AC01–06 |
| E02, E03, E10 | [PL-01–05](PLAYERS.md) | PL-BR01–06 | PL-AC01–05 |
| E04, E05 / D-05 | [VE-01–05](VENUES.md) | VE-BR01–06 | VE-AC01–05 |
| E05–E10 / D-03–06 | [MA-01–11](MATCHES.md) | MA-BR01–10 | MA-AC01–10 |
| E11 / D-06 | [PA-01–06](PAYMENTS.md) | PA-BR01–09 | PA-AC01–09 |
| PRD §6, E09–E11 | [MO-01–06](MODERATION.md) | MO-BR01–06 | MO-AC01–05 |
| E12 | [NO-01–05](NOTIFICATIONS.md) | NO-BR01–05 | NO-AC01–05 |
| E13 | [CO-01–04](COMMUNICATION.md) | CO-BR01–05 | CO-AC01–05 |
| E14 | [RE-01–05](RECOMMENDATIONS.md) | RE-BR01–06 | RE-AC01–05 |
| PRD §7, E11 / ADR data ownership | [GL-06](GL-06-DATA.md) | GL06-BR01–06 | GL06-AC01–05 |
| PRD §7–8 / Blueprint §8 | [GL-07](GL-07-MEASUREMENT.md) | GL07-BR01–05 | GL07-AC01–05 |

## Chỉ mục chi tiết theo use case

| Spec con | Quy tắc liên quan | AC kiểm chứng |
| --- | --- | --- |
| ID-01 | ID-BR01, ID-BR03, ID-BR04 | ID-AC01, ID-AC03 |
| ID-02–03 | ID-BR04, ID-BR05 | ID-AC04, ID-AC05 |
| ID-04–05 | ID-BR02, ID-BR05, ID-BR06 | ID-AC02, ID-AC05, ID-AC06 |
| PL-01–02 | PL-BR01, PL-BR02, PL-BR06 | PL-AC01, PL-AC02, PL-AC05 |
| PL-03–05 | PL-BR03, PL-BR04, PL-BR05 | PL-AC03, PL-AC04 |
| VE-01–02 | VE-BR02, VE-BR05 | VE-AC04 |
| VE-03–04 | VE-BR03, VE-BR04 | VE-AC02, VE-AC03 |
| VE-05 | VE-BR01, VE-BR06 | VE-AC01, VE-AC05 |
| MA-01 | MA-BR01, MA-BR04 | MA-AC04, MA-AC09 |
| MA-02–03 | MA-BR02 | MA-AC02 |
| MA-04–05 | MA-BR02, MA-BR03, MA-BR05 | MA-AC01, MA-AC03, MA-AC10 |
| MA-06–07 | MA-BR06, MA-BR10 | MA-AC05, MA-AC08 |
| MA-08–09 | MA-BR07, MA-BR09 | MA-AC06, PL-AC04 |
| MA-10–11 | MA-BR08, MA-BR09, MA-BR10 | MA-AC07, MA-AC08 |
| PA-01–02 | PA-BR01, PA-BR02, PA-BR03 | PA-AC01, PA-AC08 |
| PA-03 | PA-BR04, PA-BR05 | PA-AC02, PA-AC03, PA-AC04 |
| PA-04 | PA-BR06 | PA-AC05 |
| PA-05–06 | PA-BR07, PA-BR08, PA-BR09 | PA-AC06, PA-AC07, PA-AC09 |
| MO-01–02 | MO-BR01, MO-BR02 | MO-AC01, MO-AC02 |
| MO-03–04 | MO-BR03, MO-BR04, MO-BR05 | MO-AC03, MO-AC04, MO-AC02 |
| MO-05–06 | MO-BR04, MO-BR06 | MO-AC04, MO-AC05 |
| NO-01–03 | NO-BR01, NO-BR02, NO-BR03, NO-BR05 | NO-AC01, NO-AC02, NO-AC05 |
| NO-04–05 | NO-BR03, NO-BR04 | NO-AC03, NO-AC04 |
| CO-01–02 | CO-BR01, CO-BR03, CO-BR04 | CO-AC01, CO-AC03, CO-AC04 |
| CO-03–04 | CO-BR02, CO-BR03, CO-BR05 | CO-AC02, CO-AC05, MO-AC05 |
| RE-01–03 | RE-BR01, RE-BR02, RE-BR03 | RE-AC01, RE-AC02, RE-AC03 |
| RE-04–05 | RE-BR04, RE-BR05, RE-BR06 | RE-AC04, RE-AC05, GL06-AC01 |

Các spec con gộp trong file module là một gói review, không được coi mỗi dòng trên là một tài liệu đã phê duyệt độc lập.
GL-04/GL-05 trong danh mục ban đầu (hủy/hoàn; hoàn thành/đánh giá) được phân tích tại GL-03,
MA-07–10, PA-04–05 và PL-04; không tạo tài liệu trùng nguồn chính sách.

## Bộ tình huống bắt buộc

| Rủi ro | Spec/BR | AC |
| --- | --- | --- |
| Không đủ quyền / manager sân vượt quyền | GL02-BR01, GL02-BR02 | GL02-AC01, GL02-AC02 |
| Hai người tranh suất cuối | MA-BR03 | MA-AC01 |
| Trùng lịch | MA-BR02 | MA-AC02 |
| Báo/xác nhận chuyển tiền lặp | PA-BR04 | PA-AC02, PA-AC03 |
| Tiền muộn sau hết hold | MA-BR05, PA-BR05 | MA-AC10, PA-AC04 |
| Host không phản hồi | GL03-BR03 / D-13 | Kịch bản E2E-01 bên dưới |
| Hủy sau nhận tiền | MA-BR10, PA-BR06 | MA-AC08, PA-AC05 |
| Tranh chấp hoàn tiền | PA-BR07 | PA-AC06 |
| Sửa điểm danh | PL-BR05, MO-BR04 | PL-AC04, MO-AC04 |

## Nghiệm thu liên module bổ sung

- **E2E-01 [CHỐT D-13]:** Given Player báo chuyển trong hạn nhưng Host không phản hồi; When hết deadline xác nhận; Then không tự nhận tiền/JOINED, hold hết và báo chuyển vẫn còn để đối soát.
- **E2E-02 [CHỐT D-15]:** Given Host hủy kèo có tiền; When hủy được chấp nhận; Then lượt/suất kết thúc, Payments tạo nghĩa vụ hoàn đủ và Notifications không nói đã hoàn.
- **E2E-03 [CHỐT D-18]:** Given Admin sửa no-show; When correction event được phát lại; Then Matches giữ revision, Players thay tín hiệu một lần và analytics phản ánh kết quả mới.
- **E2E-04 [CHỐT D-03]:** Given verified + COMPLETE nhưng chưa duyệt quyền tổ chức; When tạo hành động công bố; Then bị từ chối ở mọi giao diện.
- **E2E-05 [CHỐT D-19/D-21]:** Given Player rời kèo có khoản hoàn; When phòng kèo thu hồi quyền; Then case/Payments vẫn truy cập theo quyền riêng, không bắt vào chat để đòi hoàn.

## Kiểm tra review cuối

- Mọi E01–E14 có owner/spec/AC.
- Q-01–Q-17 đã đóng và truy vết tới D-08–D-24.
- Một sự kiện/receipt không bị hiểu là hai trạng thái nghiệp vụ khác nhau.
- Không có bảng database/API bắt buộc hoặc ngưỡng chưa duyệt bị gọi là đã chốt.
- Gate ERD đạt; thiết kế vật lý phải giữ owner, invariant và correction history.
