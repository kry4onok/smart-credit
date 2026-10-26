import type { DocumentMetadata, DocumentType, InfoField } from "../api/types";

export function validateInfoValue(field: InfoField, value: number | string | null | undefined): string | null {
  if (value === null || value === undefined || value === "") return "Заполните поле перед отправкой ответа.";
  if (field === "APPLICATION_PURPOSE") {
    return ["WORKING_CAPITAL", "EQUIPMENT", "EXPANSION", "OTHER"].includes(String(value)) ? null : "Выберите цель кредита.";
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

export const PURPOSE_DESCRIPTION_MAX = 1000;

// validatePurposeDescription: OTHER needs a nonblank explanation before submit; any purpose caps it at 1000 characters.
export function validatePurposeDescription(purpose: string | undefined, description: string | undefined): string | null {
  const text = description ?? "";
  if ([...text].length > PURPOSE_DESCRIPTION_MAX) return `Не больше ${PURPOSE_DESCRIPTION_MAX} символов.`;
  if (purpose === "OTHER" && text.trim() === "") return "Для цели «Несколько целей или другая цель» опишите, на что пойдут деньги.";
  return null;
}
