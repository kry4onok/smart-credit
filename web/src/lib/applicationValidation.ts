import type { DocumentMetadata, DocumentType, InfoField } from "../api/types";

export function validateInfoValue(field: InfoField, value: number | string | null | undefined): string | null {
  if (value === null || value === undefined || value === "") return "Заполните поле перед отправкой ответа.";
  if (field === "APPLICATION_PURPOSE") {
    return ["WORKING_CAPITAL", "EQUIPMENT", "EXPANSION"].includes(String(value)) ? null : "Выберите цель кредита.";
  }
  if (typeof value !== "number" || !Number.isSafeInteger(value)) return "Введите целое число в допустимом диапазоне.";
  if (field === "APPLICATION_TERM_MONTHS") return value > 0 && value <= 2147483647 ? null : "Укажите целый положительный срок в месяцах.";
  if (field === "APPLICATION_AMOUNT") return value > 0 ? null : "Сумма кредита должна быть больше нуля.";
  if (field !== "COMPANY_PROFIT_12M" && value < 0) return "Значение не может быть отрицательным.";
  return null;
}

export function latestDocuments(documents: readonly DocumentMetadata[]): DocumentMetadata[] {
  const latest = new Map<DocumentType, DocumentMetadata>();
  for (const document of documents) {
    const previous = latest.get(document.document_type);
    if (!previous || Date.parse(document.uploaded_at) >= Date.parse(previous.uploaded_at)) latest.set(document.document_type, document);
  }
  return [...latest.values()];
}
