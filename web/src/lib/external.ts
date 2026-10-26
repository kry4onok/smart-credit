import type { DocumentType, LegalForm } from "../api/types";

// Where an entrepreneur gets each document. Integrations with these services are not part of
// the prototype: the link opens the official service in a new tab with a short instruction.
export type SourceHint = { how: string; service?: { label: string; url: string } };

const fnsIp = { label: "Личный кабинет ИП на nalog.ru", url: "https://lkip2.nalog.ru/" };
const fnsLlc = { label: "Личный кабинет юрлица на nalog.ru", url: "https://lkul.nalog.ru/" };
const girBo = { label: "ГИР БО — отчётность компаний", url: "https://bo.nalog.gov.ru/" };

export function documentSource(type: DocumentType, form: LegalForm | undefined): SourceHint {
  const fns = form === "IP" ? fnsIp : fnsLlc;
  switch (type) {
    case "BANK_STATEMENT":
      return { how: "Выгрузите выписку по расчётному счёту в интернет-банке в PDF или Excel." };
    case "TAX_DECLARATION_USN":
      return { how: "Скачайте декларацию УСН с отметкой о приёме в личном кабинете налогоплательщика.", service: fns };
    case "TAX_DECLARATION_3NDFL":
      return { how: "Скачайте 3-НДФЛ за нужный год в личном кабинете ИП.", service: fnsIp };
    case "PATENT":
      return { how: "Патенты отображаются в личном кабинете ИП в разделе «Патенты».", service: fnsIp };
    case "ACCOUNTING_REPORT":
      return form === "LLC"
        ? { how: "Годовая бухгалтерская отчётность ООО доступна в ГИР БО: найдите компанию по ИНН и скачайте отчёт.", service: girBo }
        : { how: "Выгрузите отчётность из бухгалтерской программы." };
    case "INCOME_LEDGER":
      return { how: "Выгрузите книгу учёта доходов и расходов из бухгалтерской программы или онлайн-бухгалтерии." };
    case "MANAGEMENT_REPORT":
      return { how: "Подойдёт отчёт о доходах и расходах из учётной системы или таблицы за указанный период." };
    case "RECEIVABLES_PAYABLES":
      return { how: "Расшифровку дебиторской и кредиторской задолженности формирует бухгалтерия на указанную дату." };
    case "BUSINESS_PLAN":
      return { how: "Опишите цель, план расходов и ожидаемый эффект — достаточно 1–2 страниц." };
  }
}

// Public registries for checking the company card before filling in the profile.
export const registryLinks = [
  { label: "Выписка ЕГРЮЛ/ЕГРИП", url: "https://egrul.nalog.ru/" },
  { label: "Прозрачный бизнес", url: "https://pb.nalog.ru/" },
];
