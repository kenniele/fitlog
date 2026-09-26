import { ScanLine } from "lucide-react";
import type { BodyFatEstimate as Estimate } from "@/lib/types";
import { formatDate, formatNumber } from "@/lib/format";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

const unavailable: Record<Exclude<Estimate["status"], "estimated">, string> = {
  no_inbody: "Добавьте InBody с весом и процентом или массой жира — он станет отправной точкой.",
  incomplete_inbody: "В последнем InBody нужны вес и процент или масса жира. Более ранний замер не подставляется вместо него.",
  no_tdee: "Для расчёта нужен настроенный суточный расход TDEE — тот же, что используется в отчёте о дефиците.",
  no_complete_days: "После замера ещё нет полных дней для расчёта. День InBody и сегодняшний день не учитываются.",
  stale_inbody: "После последнего InBody прошло больше года. Добавьте новый замер, чтобы обновить отправную точку.",
  insufficient_nutrition: "Недостаточно данных: нужны калории хотя бы за 80% полных дней после замера. Пропуски не считаются нулевым питанием.",
  outside_model: "Накопленный баланс выходит за пределы этой простой модели. Новый InBody поможет обновить оценку.",
};

export function BodyFatEstimateCard({ estimate }: { estimate?: Estimate | null }) {
  if (!estimate) return null;
  const ready = estimate.status === "estimated";
  const reason = estimate.status === "estimated" ? null : unavailable[estimate.status];
  const deficit = estimate.estimated_deficit_kcal;
  return <Card className="p-5" aria-labelledby="body-fat-estimate-title">
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="flex items-center gap-2"><ScanLine className="size-4 text-accent" aria-hidden /><h2 id="body-fat-estimate-title" className="text-sm font-semibold text-ink">Ожидаемый процент жира</h2></div>
      <Badge>Приблизительная оценка</Badge>
    </div>
    {ready ? <>
      <p className="mt-4 text-3xl font-semibold tabular-nums text-ink" aria-label="Диапазон ожидаемого процента жира">{formatNumber(estimate.lower_percent, { maximumFractionDigits: 1 })}–{formatNumber(estimate.upper_percent, { maximumFractionDigits: 1 })}<span className="ml-1 text-lg text-muted">%</span></p>
      <p className="mt-1 text-sm text-muted">Центральный сценарий — около {formatNumber(estimate.percent, { maximumFractionDigits: 1 })}%. Это расчёт по калориям, а не новое измерение.</p>
      <div className="mt-4 grid gap-3 rounded-control border border-line bg-white/[.025] p-3 sm:grid-cols-3">
        <div><p className="text-xs text-muted">Отправная точка</p><p className="mt-1 text-sm font-medium">{formatNumber(estimate.baseline_percent, { maximumFractionDigits: 1 })}% · InBody {formatDate(estimate.measured_at, "dd.MM.yyyy", estimate.timezone)}</p></div>
        <div><p className="text-xs text-muted">{typeof deficit === "number" && deficit < 0 ? "Накопленный профицит" : "Накопленный дефицит"}{estimate.missing_days > 0 ? " · оценка" : ""}</p><p className="mt-1 text-sm font-medium">{formatNumber(typeof deficit === "number" ? Math.abs(deficit) : null, { maximumFractionDigits: 0 })} ккал</p></div>
        <div><p className="text-xs text-muted">Суточный расход TDEE</p><p className="mt-1 text-sm font-medium">{formatNumber(estimate.tdee_kcal, { maximumFractionDigits: 0 })} ккал</p></div>
      </div>
    </> : <p role="status" className="mt-3 text-sm leading-6 text-muted">{reason}</p>}
    {estimate.total_days > 0 && estimate.status !== "stale_inbody" && estimate.status !== "no_tdee" && <div className="mt-3 text-xs leading-5 text-muted">
      <p>{formatDate(estimate.from)} — {formatDate(estimate.through)} · {estimate.timezone}. Питание: {estimate.observed_days} из {estimate.total_days} дней ({formatNumber(estimate.coverage_percent, { maximumFractionDigits: 0 })}%). Период считается от InBody независимо от фильтра страницы.</p>
      {ready && estimate.missing_days > 0 && <p className="mt-1">Пропущено дней: {estimate.missing_days}. Для них взято среднее питание известных дней с более широким допуском. По записанным дням баланс: {formatNumber(estimate.observed_deficit_kcal, { maximumFractionDigits: 0 })} ккал (плюс — дефицит, минус — профицит).</p>}
    </div>}
    <details className="mt-4 border-t border-line pt-3 text-xs leading-5 text-muted">
      <summary className="cursor-pointer font-medium text-ink">Как считается диапазон</summary>
      <div className="mt-2 space-y-2">
        <p>Из исходной жировой массы вычитается накопленный дефицит / 7700 ккал на кг. При профиците масса увеличивается. Процент пересчитывается от нового расчётного веса; безжировую массу условно считаем неизменной.</p>
        <p>Границы — сценарии с TDEE ±15%, питанием ±10% и исходным жиром ±2 процентных пункта. Для пропущенных дней допуск питания ±50%. Это выбранные допуски модели, а не статистический доверительный интервал или подтверждённая погрешность InBody.</p>
        <p>Берём полные дни после даты замера до вчера. Даже день с калориями может быть заполнен не полностью. Текущий TDEE применяется ко всему периоду; изменения расхода, воды и мышечной массы модель не восстанавливает.</p>
        <p>Такой пересчёт даёт только ориентир. <a className="text-accent underline underline-offset-2" href="https://pubmed.ncbi.nlm.nih.gov/17848938/" target="_blank" rel="noreferrer">Ограничения правила пересчёта калорий в массу</a>.</p>
      </div>
    </details>
  </Card>;
}
