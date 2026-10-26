import type { ScoreResult } from "../../api/types";
import { factorLabel, reasonLabel } from "../../lib/labels";

const bands = [
  { grade: "E", from: 0, to: 350, note: "отказ" },
  { grade: "D", from: 350, to: 500, note: "ручной разбор" },
  { grade: "C", from: 500, to: 650, note: "ручной разбор" },
  { grade: "B", from: 650, to: 800, note: "оферта после проверки" },
  { grade: "A", from: 800, to: 1000, note: "оферта после проверки" },
];

// ScorePanel shows the risk score on the full 0..1000 scale with grade bands and its decomposition.
export function ScorePanel({ score }: { score: ScoreResult | undefined }) {
  if (!score) {
    return <p className="muted">Оценки нет: скоринг ещё не выполнен или источник был недоступен (причина — в истории дела).</p>;
  }
  if (score.assessment_status !== "ASSESSED" || score.score === undefined) {
    return (
      <div className="note note-attention">
        Не удалось оценить заявку: {score.assessment_status === "INSUFFICIENT_DATA" ? "недостаточно данных" : "проверьте исходные данные"}.
        {score.data_issues.length > 0 && (
          <ul>
            {score.data_issues.map((d) => (
              <li key={d.code + d.field_path}>
                {d.code}: <code>{d.field_path}</code>
              </li>
            ))}
          </ul>
        )}
      </div>
    );
  }
  const baseline = score.baseline_score ?? 500;
  const maxPoints = Math.max(1, ...(score.factor_contributions ?? []).map((f) => Math.abs(f.points)));
  return (
    <div className="score">
      <div className="score-head">
        <span className={`grade grade-${score.grade}`}>{score.grade}</span>
        <div>
          <strong className="score-value">{score.score}</strong>
          <span className="muted"> из 1000 — чем выше, тем ниже риск</span>
        </div>
      </div>
      <div className="scale" role="img" aria-label={`Оценка ${score.score} из 1000, категория ${score.grade}`}>
        {bands.map((b) => (
          <span key={b.grade} className={`band band-${b.grade}${b.grade === score.grade ? " band-on" : ""}`} style={{ flexGrow: b.to - b.from }}>
            <b>{b.grade}</b>
            <small>{b.note}</small>
          </span>
        ))}
        <span className="scale-marker" style={{ left: `${(score.score / 1000) * 100}%` }} />
      </div>

      {score.reasons.length > 0 && (
        <ul className="reasons">
          {score.reasons.map((r) => (
            <li key={r.code} className={r.direction === "INCREASES_RISK" ? "risk-up" : "risk-down"}>
              <span aria-hidden>{r.direction === "INCREASES_RISK" ? "▲" : "▼"}</span>
              <span>
                <strong>{reasonLabel[r.code] ?? "Фактор кредитного риска"}</strong>
                {r.description && r.description !== reasonLabel[r.code] && <small>{r.description}</small>}
              </span>
              <span className="reason-points">
                {r.direction === "INCREASES_RISK" ? "−" : "+"}
                {r.contribution}
              </span>
            </li>
          ))}
        </ul>
      )}

      {score.factor_contributions && score.factor_contributions.length > 0 && (
        <div className="factors">
          <p className="muted">
            Разложение: база {baseline} + вклады факторов = {score.score}
          </p>
          {score.factor_contributions.map((f) => (
            <div key={f.factor_name} className="factor">
              <span>{factorLabel[f.factor_name] ?? "Фактор оценки"}</span>
              <span className="factor-bar">
                <span
                  className={f.points >= 0 ? "pos" : "neg"}
                  style={{ width: `${(Math.abs(f.points) / maxPoints) * 50}%`, [f.points >= 0 ? "left" : "right"]: "50%" }}
                />
              </span>
              <span className="num">
                {f.points > 0 ? "+" : ""}
                {f.points}
              </span>
            </div>
          ))}
        </div>
      )}
      {score.as_of_date && <p className="score-meta">Данные на {score.as_of_date}</p>}
    </div>
  );
}
