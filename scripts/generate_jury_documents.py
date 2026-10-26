#!/usr/bin/env python3
"""Build fictional jury attachments from the local demo catalogue (no network)."""

import argparse
import calendar
import hashlib
import json
from datetime import date
from pathlib import Path
from xml.sax.saxutils import escape

from reportlab import rl_config
from reportlab.lib import colors
from reportlab.lib.styles import ParagraphStyle
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle

ROOT = Path(__file__).resolve().parents[1]
LABELS = {
    "BANK_STATEMENT": "Банковская выписка",
    "MANAGEMENT_REPORT": "Управленческий отчёт",
    "ACCOUNTING_REPORT": "Бухгалтерская отчётность",
    "RECEIVABLES_PAYABLES": "Дебиторская и кредиторская задолженность",
    "TAX_DECLARATION_USN": "Демонстрационная декларация УСН",
    "TAX_DECLARATION_3NDFL": "Демонстрационная декларация 3-НДФЛ",
    "INCOME_LEDGER": "Книга учёта доходов и расходов",
    "PATENT": "Демонстрационный патент",
    "BUSINESS_PLAN": "Бизнес-план использования кредита",
}
WARNING = "СИНТЕТИЧЕСКИЕ ДАННЫЕ • ТОЛЬКО ДЛЯ ДЕМОНСТРАЦИИ SMART CREDIT"


def money(value):
    sign = "-" if value < 0 else ""
    value = abs(value)
    return f"{sign}{value // 100}.{value % 100:02d}"


def month_shift(day, delta):
    index = day.year * 12 + day.month - 1 + delta
    return date(index // 12, index % 12 + 1, 1)


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


class Pack:
    def __init__(self, output, as_of):
        self.output, self.as_of = output, as_of
        self.start = month_shift(as_of.replace(day=1), -12)
        end = month_shift(self.start, 11)
        self.end = end.replace(day=calendar.monthrange(end.year, end.month)[1])
        self.annual = (f"{as_of.year - 1}-01-01", f"{as_of.year - 1}-12-31")
        self.period = (self.start.isoformat(), self.end.isoformat())
        self.entries, self.scenarios = [], []
        self.body = ParagraphStyle("body", fontName="Jury", fontSize=9, leading=13, spaceAfter=5)
        self.small = ParagraphStyle("small", parent=self.body, fontSize=7.3, leading=10)
        self.title = ParagraphStyle("title", parent=self.body, fontName="JuryBold", fontSize=18, leading=22, spaceAfter=14)

    def pdf(self, path, title, lines, rows=None):
        def footer(canvas, doc):
            canvas.setFont("Jury", 7)
            canvas.setFillColor(colors.HexColor("#697386"))
            canvas.drawString(42, 26, "SMART CREDIT / MOCK / Не является официальным документом")
            canvas.drawRightString(553, 26, str(doc.page))
        story = [Paragraph(escape(WARNING), self.small), Spacer(1, 8), Paragraph(escape(title), self.title)]
        story += [Paragraph(escape(line), self.body) if line else Spacer(1, 6) for line in lines]
        if rows:
            table = Table([[Paragraph(escape(str(v)), self.small) for v in row] for row in rows], repeatRows=1, hAlign="LEFT")
            table.setStyle(TableStyle([
                ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#e9eef5")),
                ("VALIGN", (0, 0), (-1, -1), "TOP"),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 7),
                ("TOPPADDING", (0, 0), (-1, -1), 7),
                ("LINEBELOW", (0, 0), (-1, -1), 0.3, colors.HexColor("#ccd3dc")),
            ]))
            story += [Spacer(1, 10), table]
        SimpleDocTemplate(str(path), rightMargin=42, leftMargin=42, topMargin=38, bottomMargin=43,
                          title=title, author="Smart Credit synthetic demo", pageCompression=1).build(story, onFirstPage=footer, onLaterPages=footer)

    def document(self, folder, company, kind, coverage, lines, rows=None, suffix="", code=None, regime="all"):
        title = LABELS[kind]
        header = [f"Тип документа: {kind}", f"Владелец: {company['name']}", f"ИНН из демо-каталога: {company['inn']}",
                  f"Период покрытия: {coverage[0]} - {coverage[1]}", "Валюта: RUB. Все суммы в рублях.",
                  "Это вымышленный макет. Подпись, печать и подтверждение государственного органа отсутствуют.", ""]
        stem = kind.lower() + suffix
        path = self.output / folder / stem
        path.parent.mkdir(parents=True, exist_ok=True)
        content = "\n".join([title, WARNING, *header, *lines])
        if rows:
            content += "\n\n" + "\n".join(" | ".join(map(str, row)) for row in rows)
        path.with_suffix(".txt").write_text(content + "\n", encoding="utf-8")
        self.pdf(path.with_suffix(".pdf"), title, header + lines, rows)
        requirement_id = f"{code.lower()}:{coverage[0]}:{coverage[1]}:{regime.lower()}" if code else None
        self.entries.append({"scenario": folder, "inn": company["inn"], "document_type": kind,
                             "coverage_from": coverage[0], "coverage_to": coverage[1],
                             "requirement_code": code, "requirement_id_example": requirement_id,
                             "tax_regime": regime, "files": [str(path.with_suffix(ext).relative_to(self.output)) for ext in (".txt", ".pdf")]})

    def statement(self, folder, c):
        receipts = c["monthly_receipts_kopecks"]
        surplus = receipts * c["margin_bps"] // 10000
        variable = receipts - surplus - 25_000_000
        opening = c["initial_cash_kopecks"]
        # The loss case needs explicit financing to avoid an impossible negative cash balance.
        funding = max(0, -(opening + 12 * surplus))
        operations = []
        for i in range(12):
            month = month_shift(self.start, i)
            def add(day, direction, amount, counterparty, purpose):
                operations.append({"id": f"DEMO-{c['number']:02}-{len(operations)+1:04}", "date": month.replace(day=day).isoformat(),
                                   "direction": direction, "amount_kopecks": amount, "counterparty": counterparty, "purpose": purpose})
            if i == 0 and funding:
                add(1, "Поступление", funding, "Демо-учредитель", "Вклад в имущество: финансирование убытка, не выручка")
            largest = receipts * c["largest_customer_share_bps"] // 10000
            remaining = receipts - largest
            shares = [largest] + [remaining // 9 + (1 if n < remaining % 9 else 0) for n in range(9)]
            for n, amount in enumerate(shares):
                add(2 + n, "Поступление", amount, f"Демо-покупатель {n+1:02}", "Оплата реализации в синтетическом месяце")
            for day, amount, party, purpose in [
                (15, variable, "Демо-поставщик", "Переменные расходы"),
                (18, 15_000_000, "Демо-получатели", "Постоянные расходы"),
                (20, 6_000_000, "Демо-бюджет", "Модельный налоговый платёж, не расчёт по законодательству"),
                (23, 2_000_000, "Демо-сервис", "Текущее обслуживание"),
                (25, 2_000_000, "Демо-собственник", "Выплата собственнику; не расход при расчёте прибыли"),
            ]:
                add(day, "Списание", amount, party, purpose)
        credits = sum(o["amount_kopecks"] for o in operations if o["direction"] == "Поступление")
        debits = sum(o["amount_kopecks"] for o in operations if o["direction"] == "Списание")
        closing = opening + credits - debits
        lines = ["ДЕМОНСТРАЦИОННАЯ ВЫПИСКА ПО СЧЁТУ", "", WARNING,
                 "Вымышленный формат SMART_CREDIT_DEMO_STATEMENT_TXT_V1; не банковский стандарт.", "",
                 "Тип документа: BANK_STATEMENT", f"Владелец счёта: {c['name']}", f"Идентификатор счёта: DEMO-ACCOUNT-{c['number']:02}",
                 f"Начало периода: {self.period[0]}", f"Конец периода: {self.period[1]}", "Валюта: RUB",
                 "Суммы указаны в рублях; десятичный разделитель — точка.", "", "ОСТАТКИ И ОБОРОТЫ",
                 f"Входящий остаток: {money(opening)}", f"Всего поступлений: {money(credits)}",
                 f"Всего списаний: {money(debits)}", f"Исходящий остаток: {money(closing)}", "", "ОПЕРАЦИИ", ""]
        for o in operations:
            lines += [f"Операция: {o['id']}", f"Дата: {o['date']}", f"Направление: {o['direction']}",
                      f"Сумма: {money(o['amount_kopecks'])}", f"Контрагент: {o['counterparty']}", f"Назначение: {o['purpose']}", ""]
        lines += ["ПРИМЕЧАНИЕ", "Поступления не являются автоматически выручкой, а чистый денежный поток не является бухгалтерской прибылью.",
                  f"В этой модели отдельно задано: операционные поступления {money(receipts * 12)}; вклад учредителя {money(funding)}.",
                  "Финансирование показано только в файле и не добавлено во встроенный источник оценки.",
                  "Остаток кредитного долга и будущие платежи приведены в управленческом отчёте; из этой выписки они не выводятся."]
        path = self.output / folder / "bank_statement.txt"
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text("\n".join(lines) + "\n", encoding="utf-8")
        self.entries.append({"scenario": folder, "inn": c["inn"], "document_type": "BANK_STATEMENT", "coverage_from": self.period[0],
                             "coverage_to": self.period[1], "requirement_code": "CASH_FLOW",
                             "requirement_id_example": f"cash_flow:{self.period[0]}:{self.period[1]}:all",
                             "files": [str(path.relative_to(self.output))], "expected_parser": {
                                 "opening_balance_kopecks": opening, "total_credits_kopecks": credits, "total_debits_kopecks": debits,
                                 "closing_balance_kopecks": closing, "operations": len(operations)}})
        return receipts, variable, surplus, funding

    def company(self, folder, c, regimes=None, changed=False, extension=False):
        regimes = regimes or [c["tax_regime"]]
        ip = len(c["inn"]) == 12
        receipts, variable, surplus, funding = self.statement(folder, c)
        rows = [["Месяц", "Поступления", "Денежные расходы*", "Остаток потока"]]
        for i in range(12):
            rows.append([month_shift(self.start, i).strftime("%Y-%m"), money(receipts), money(receipts-surplus), money(surplus)])
        rows.append(["Итого", money(receipts*12), money((receipts-surplus)*12), money(surplus*12)])
        profit = (surplus + 2_000_000) * 12
        lines = [f"Налоговые режимы сценария: {', '.join(regimes)}.",
                 "Допущение макета: реализация оплачена в том же месяце, амортизация и изменение запасов равны нулю.",
                 f"Выручка за 12 месяцев: {money(receipts*12)}; управленческая прибыль: {money(profit)}.",
                 "Прибыль исключает выплаты собственнику; остаток денежного потока ниже на сумму этих выплат.",
                 f"Ежемесячно: переменные расходы {money(variable)}, постоянные 150000.00, налоговый платёж 60000.00, обслуживание 20000.00, выплата собственнику 20000.00.",
                 f"Остаток кредитных обязательств на {self.as_of}: {money(c['debt_kopecks'])}.",
                 f"Концентрация крупнейшего покупателя: {c['largest_customer_share_bps']/100:.2f}% операционных поступлений.",
                 f"Дополнительное финансирование от учредителя в выписке: {money(funding)}. Не включено в выручку.",
                 "*Денежные расходы включают выплаты собственнику. Налоговый платёж перенесён из DemoSources; это не налоговый расчёт."]
        self.document(folder, c, "MANAGEMENT_REPORT", self.period, lines, rows, code="BUSINESS_RESULT" if ip else "MANAGEMENT_REPORT")
        # Future debt schedule is a separate attachment to the management report, not a new upload type.
        balance = c["debt_kopecks"]
        schedule = []
        for i in range(1, 25):
            principal = balance // (25-i)
            interest = balance * 150 // 10000
            balance -= principal
            schedule.append({"month": month_shift(self.as_of.replace(day=1), i).strftime("%Y-%m"),
                             "principal_kopecks": principal, "interest_kopecks": interest, "payment_kopecks": principal + interest})
        write_json(self.output / folder / "scenario.json", {"synthetic": True, "extension_requires_profile_change": extension,
                   "company": c, "tax_regimes": regimes, "reference_date": str(self.as_of),
                   "financial_period": {"from": self.period[0], "to": self.period[1]},
                   "management_revenue_kopecks": receipts*12, "management_profit_kopecks": profit,
                   "cash_surplus_kopecks": surplus*12, "additional_owner_funding_kopecks": funding,
                   "future_debt_schedule": schedule})
        periods = [{"from": c["registered_at"], "regimes": regimes, "vat_status": "UNKNOWN"}]
        if changed:
            periods = [
                {"from": c["registered_at"], "to": f"{self.as_of.year-1}-12-31", "regimes": ["USN_INCOME"], "vat_status": "UNKNOWN"},
                {"from": f"{self.as_of.year}-01-01", "regimes": ["PSN"], "vat_status": "UNKNOWN"},
            ]
        write_json(self.output / folder / "profile.json", {
            "profile_schema_version": 2, "inn": c["inn"], "name": c["name"], "legal_form": "IP" if ip else "LLC",
            "registered_at": c["registered_at"], "okved": c["okved"], "tax_periods": periods,
            "financial_period": {"from": self.period[0], "to": self.period[1]}, "financial_basis": "MANAGEMENT",
            "obligations_as_of": str(self.as_of), "revenue_12m": {"amount_kopecks": receipts*12, "currency": "RUB"},
            "obligations": {"amount_kopecks": c["debt_kopecks"], "currency": "RUB"},
        })
        if not ip:
            # Separate previous calendar year, deliberately not labelled as the rolling period.
            annual_cash = c["initial_cash_kopecks"] + surplus*12 + funding
            assets = annual_cash + 50_000_000 + 20_000_000 + 10_000_000
            creditors = 5_000_000
            equity = assets - c["debt_kopecks"] - creditors
            accounting = [["Статья", "Сумма, RUB"], ["Основные средства", money(50_000_000)], ["Запасы", money(20_000_000)],
                          ["Дебиторская задолженность", money(10_000_000)], ["Денежные средства", money(annual_cash)], ["Активы, итого", money(assets)],
                          ["Капитал (включая накопленный результат)", money(equity)], ["Кредиты и займы", money(c["debt_kopecks"])],
                          ["Кредиторская задолженность", money(creditors)], ["Пассивы, итого", money(equity+c["debt_kopecks"]+creditors)],
                          ["Выручка за календарный год", money(receipts*12)], ["Расходы, включая модельный налог", money(receipts*12-profit)], ["Чистая прибыль / убыток", money(profit)]]
            self.document(folder, c, "ACCOUNTING_REPORT", self.annual,
                          ["Упрощённый учебный баланс и отчёт о результатах. Не форма ФНС и не выгрузка ГИР БО.",
                           "Допущение: для предыдущего календарного года принят тот же постоянный месячный поток; это отдельный годовой макет.",
                           "Выплаты собственнику уменьшают капитал. Баланс составлен по синтетическим остаткам."], accounting, code="BUSINESS_RESULT")
            self.document(folder, c, "RECEIVABLES_PAYABLES", (str(self.as_of), str(self.as_of)),
                          ["Остатки на дату запроса. Допущение: с конца прошлого календарного года остатки расчётов не изменились.",
                           "Просроченная торговая задолженность: 0.00. Кредитная история проверяется отдельным мок-источником.",
                           f"Кредитные обязательства {money(c['debt_kopecks'])} не включены в торговую кредиторскую задолженность."],
                          [["Вид", "Вымышленный контрагент", "Сумма, RUB"], ["Дебиторская", "Демо-покупатель А", "60000.00"],
                           ["Дебиторская", "Демо-покупатель Б", "40000.00"], ["Дебиторская, итог", "", "100000.00"],
                           ["Кредиторская", "Демо-поставщик", "50000.00"]], code="RECEIVABLES_PAYABLES")
        if "PSN" in regimes:
            self.document(folder, c, "PATENT", (f"{self.as_of.year}-01-01", f"{self.as_of.year}-12-31"),
                          [f"Номер макета: DEMO-PATENT-{self.as_of.year}-001.", "Вид деятельности: учебная розничная торговля (ОКВЭД 47.11).",
                           "Территория: вымышленная территория Demo. Орган выдачи: DEMO, государственная выдача не заявляется.",
                           "Статус в сценарии: действует весь указанный календарный год.",
                           "Потенциальный доход и стоимость патента не используются для скоринга и не подтверждают фактическую выручку."], code="PATENT_RIGHT", regime="PSN")
            self.entries[-1]["requirement_id_example"] = f"patent_right:{self.as_of}:{self.as_of}:psn"
        tax_regimes = [r for r in regimes if r != "PSN"]
        for regime in tax_regimes:
            # Split mixed activities explicitly: a full turnover must not be counted twice.
            factor = 2 if "PSN" in regimes and not changed else 1
            revenue = receipts*12 // factor
            expenses = (receipts*12-profit) // factor
            kind = "TAX_DECLARATION_3NDFL" if regime == "OSNO" else "TAX_DECLARATION_USN"
            if kind == "TAX_DECLARATION_3NDFL" and not ip:
                continue
            self.document(folder, c, kind, self.annual,
                          [f"Режим: {regime}. Вымышленное представление данных, не утверждённая налоговая форма.",
                           f"Доходы от деятельности: {money(revenue)}; модельные расходы: {money(expenses)}.",
                           "Для совмещения УСН и ПСН в макете принят равный раздел оборота между видами деятельности." if factor == 2 else "Для годового макета принят постоянный месячный оборот.",
                           "Расходы показаны справочно; налоговая база, ставка, вычеты и сумма налога намеренно не рассчитываются.",
                           "Файл проверяет загрузку и покрытие периода. Он не подтверждает исполнение налоговых обязательств."],
                          suffix="_" + regime.lower(), code="TAX_DECLARATION", regime=regime)
        if ip:
            ledger_regimes = ["USN_INCOME", "PSN"] if changed else regimes
            for regime in ledger_regimes:
                coverage = self.period
                selected = list(range(12))
                if changed:
                    split = date(self.as_of.year, 1, 1)
                    selected = [i for i in selected if (month_shift(self.start, i) < split) == (regime == "USN_INCOME")]
                    first, last = month_shift(self.start, selected[0]), month_shift(self.start, selected[-1])
                    coverage = (first.isoformat(), last.replace(day=calendar.monthrange(last.year, last.month)[1]).isoformat())
                factor = 2 if len(regimes) > 1 and not changed else 1
                ledger = [["Месяц", "Доходы, RUB", "Расходы, RUB"]]
                ledger += [[month_shift(self.start, i).strftime("%Y-%m"), money(receipts//factor), money((receipts*12-profit)//12//factor)] for i in selected]
                ledger.append(["Итого", money(receipts*len(selected)//factor), money((receipts*12-profit)//12*len(selected)//factor)])
                self.document(folder, c, "INCOME_LEDGER", coverage,
                              [f"Режим: {regime}. Сводный учебный регистр с месячной агрегацией.",
                               "Основание: синтетические оплаты покупателей и расходы. Расходы не включают выплаты собственнику.",
                               "При ПСН и УСН-доходы расходы справочные; признание расходов в налоговом учёте не моделируется.",
                               "Совмещение: каждому режиму отнесено 50% потока." if factor == 2 else "В книгу включён только указанный период режима."], ledger,
                              suffix="_" + regime.lower(), code="INCOME_LEDGER", regime=regime)
        amount = c["suggested_amount_kopecks"]
        self.document(folder, c, "BUSINESS_PLAN", (str(self.as_of), str(self.as_of)),
                      ["Цель: пополнение оборотных средств (WORKING_CAPITAL).",
                       f"Запрашиваемая сумма: {money(amount)}; срок: {c['suggested_term_months']} месяцев.",
                       f"Бюджет: закупки {money(amount*7//10)}; операционный резерв {money(amount-amount*7//10)}.",
                       "Прогноз: повторение прошлых 12 месяцев без роста. Стресс в приложении рассчитывается отдельно.",
                       "План не гарантирует одобрение. Для убыточного сценария служит объяснением заявки при отрицательном потоке.",
                       "Загружать только если тип BUSINESS_PLAN есть в продукте или активном запросе; политика v2 сама его не запрашивает."])
        self.scenarios.append({"folder": folder, "name": c["name"], "email": c["email"], "inn": c["inn"],
                               "title": c["title"], "expected_behavior": c["description"], "extension": extension})


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--as-of", type=date.fromisoformat, default=date(2026, 10, 26))
    parser.add_argument("--output", type=Path, default=ROOT / "demo/jury-documents")
    parser.add_argument("--font", type=Path, default=Path("/System/Library/Fonts/Supplemental/Arial.ttf"))
    parser.add_argument("--bold-font", type=Path, default=Path("/System/Library/Fonts/Supplemental/Arial Bold.ttf"))
    args = parser.parse_args()
    rl_config.invariant = True
    pdfmetrics.registerFont(TTFont("Jury", str(args.font)))
    pdfmetrics.registerFont(TTFont("JuryBold", str(args.bold_font)))
    pack = Pack(args.output, args.as_of)
    companies = json.loads((ROOT / "pkg/demodata/companies.json").read_text())
    slugs = ["llc_approval", "ip_patent", "llc_liquidation", "llc_grade_b", "llc_counteroffer", "llc_cash_gap", "llc_loss", "llc_manual_review", "llc_source_unavailable"]
    for c, slug in zip(companies, slugs, strict=True):
        pack.company(f"{c['number']:02}_{slug}", c)
    ip = next(c for c in companies if len(c["inn"]) == 12)
    for slug, regimes, changed in [("ip_usn_income", ["USN_INCOME"], False), ("ip_usn_expense", ["USN_INCOME_EXPENSE"], False),
                                   ("ip_osno", ["OSNO"], False), ("ip_usn_psn", ["USN_INCOME_EXPENSE", "PSN"], False),
                                   ("ip_regime_changed", ["USN_INCOME", "PSN"], True)]:
        pack.company("extended/" + slug, ip, regimes, changed=changed, extension=True)
    negative = args.output / "negative"
    negative.mkdir(parents=True, exist_ok=True)
    original = (args.output / "01_llc_approval/bank_statement.txt").read_text()
    (negative / "statement_bad_totals.txt").write_text(original.replace("Всего поступлений: 12000000.00", "Всего поступлений: 12000000.01"))
    # A valid short statement isolates coverage matching from parser arithmetic.
    (negative / "statement_wrong_period.txt").write_text("\n".join([
        "ДЕМОНСТРАЦИОННАЯ ВЫПИСКА ПО СЧЁТУ", WARNING, "Тип документа: BANK_STATEMENT", "Владелец счёта: ООО «Северная пекарня»",
        "Идентификатор счёта: DEMO-SHORT", f"Начало периода: {pack.period[1][:7]}-01", f"Конец периода: {pack.period[1]}",
        "Валюта: RUB", "ОСТАТКИ И ОБОРОТЫ", "Входящий остаток: 0.00", "Всего поступлений: 0.00", "Всего списаний: 0.00",
        "Исходящий остаток: 0.00", "ОПЕРАЦИИ", "ПРИМЕЧАНИЕ", "Пустая учебная выписка за один месяц; не покрывает годовой запрос.", ""]))
    (negative / "patent_expired.txt").write_text(f"{WARNING}\nТип документа: PATENT\nВладелец: {ip['name']}\nИНН: {ip['inn']}\nПериод: {args.as_of.year-1}-01-01 - {args.as_of.year-1}-12-31\nМакет просроченного патента. Не покрывает дату запроса.\n")
    (negative / "invalid_signature.pdf").write_text("Это намеренно текстовый файл с неверным расширением PDF. Только для проверки отказа загрузки.\n")
    (negative / "empty.txt").write_bytes(b"")
    negative_cases = [
        {"file": "negative/statement_bad_totals.txt", "layer": "parser", "expected": "INCONSISTENT_TOTALS", "upload_accepts": True},
        {"file": "negative/statement_wrong_period.txt", "layer": "coverage", "expected": "period does not cover requirement", "parser_accepts": True},
        {"file": "negative/patent_expired.txt", "layer": "coverage", "expected": "period does not cover reference date"},
        {"file": "negative/invalid_signature.pdf", "layer": "upload", "expected": "INVALID_DOCUMENT"},
        {"file": "negative/empty.txt", "layer": "upload", "expected": "INVALID_DOCUMENT"},
    ]
    manifest = {"schema_version": 1, "synthetic": True, "reference_date": str(args.as_of),
                "source": "pkg/demodata/companies.json + pkg/assessment/demo.go", "scenarios": pack.scenarios,
                "documents": pack.entries, "negative_cases": negative_cases,
                "notice": "Expected behavior is code-derived, not evidence of a live E2E run. Get requirement_id from the current application."}
    for entry in pack.entries:
        entry["sha256"] = {f: hashlib.sha256((args.output / f).read_bytes()).hexdigest() for f in entry["files"]}
    write_json(args.output / "manifest.json", manifest)
    pack.pdf(args.output / "jury-guide.pdf", "Документы для проверки жюри", [
        f"Дата комплекта: {args.as_of}. Выписки: {pack.period[0]} - {pack.period[1]}. Годовые документы: {args.as_of.year-1} год.",
        "Откройте папку компании. bank_statement.txt - выписка с операциями. Остальные документы доступны в PDF и TXT; достаточно загрузить один вариант.",
        "ИП на патенте: 02_ip_patent/patent.pdf. При NEEDS_INFO загрузите его в слот действующего патента и отдельно отправьте ответ банку.",
        "При v2 берите requirement_id из текущего запроса, а даты покрытия - из документа. Файл должен покрывать весь запрошенный период.",
        "Оценка использует встроенные синтетические источники. Загрузка документа не меняет поток, балл или решение автоматически. PDF не разбирается в финансовые показатели.",
        "extended содержит пять вариантов ИП: УСН-доходы, УСН-доходы-расходы, ОСНО, совмещение УСН/ПСН и смена режима. Перед созданием новой заявки нужен соответствующий профиль; это не дополнительные учётные записи.",
        "negative содержит намеренно ошибочные файлы. Подробные ожидаемые ошибки, ограничения и порядок действий - в README.md и manifest.json.",
        "Результаты ниже ожидаются по текущему коду и настройкам моков. Они не являются отчётом о выполненном сквозном прогоне.",
    ], [["Папка", "Ожидаемое поведение"]] + [[s["folder"], s["title"]] for s in pack.scenarios if not s["extension"]])
    print(json.dumps({"documents": len(pack.entries), "pdf": len(list(args.output.rglob("*.pdf")))-1,
                      "scenarios": len(pack.scenarios), "output": str(args.output)}, ensure_ascii=False))


if __name__ == "__main__":
    main()
