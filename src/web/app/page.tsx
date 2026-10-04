const foundations = [
  "Tìm kèo phù hợp theo thời gian, vị trí và trình độ",
  "Tạo và quản lý kèo với sức chứa rõ ràng",
  "Xây dựng độ tin cậy qua check-in và lịch sử chơi",
  "Đề xuất có lý do, không chỉ là một con số",
];

export default function HomePage() {
  return (
    <main>
      <section className="hero">
        <p className="eyebrow">BADMINTON HUB · FOUNDATION</p>
        <h1>Muốn đánh cầu — luôn có kèo phù hợp.</h1>
        <p className="lead">
          Nền tảng giúp người chơi tìm đúng người, đúng sân, đúng thời gian và đúng trình độ.
        </p>
        <div className="actions">
          <button type="button">Tìm kèo</button>
          <button className="secondary" type="button">Tạo kèo</button>
        </div>
      </section>
      <section aria-labelledby="foundation-title" className="foundation">
        <h2 id="foundation-title">MVP tập trung vào matchmaking</h2>
        <ul>
          {foundations.map((item) => <li key={item}>{item}</li>)}
        </ul>
      </section>
    </main>
  );
}
