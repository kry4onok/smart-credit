import { auditActionLabel, bankStatusLabel, checkCodeLabel, companyStatusLabel, purposeLabel, reasonLabel, ruleLabel } from "./labels";

// Display names for existing catalogue entries, including saved application snapshots.
// API identifiers and persisted data stay unchanged.
const productNames: Record<string, string> = {
  "Демо: денежный поток": "Кредит по денежному потоку",
  "Демо: профиль ИП/ООО": "Кредит для ИП и ООО",
  "Демо: профиль ИП и ООО": "Кредит для ИП и ООО",
};

export function productName(name: string): string {
  return productNames[name] ?? name;
}

export function productDescription(code: string, description: string): string {
  if (code === "SME_CASHFLOW_V2") return "Условия кредита с учётом поступлений, расходов и платежей вашего бизнеса.";
  if (code === "SME_PROFILE_V2") return "Финансирование для индивидуальных предпринимателей и компаний с учётом особенностей бизнеса.";
  return description;
}

// Translate system-generated prose at the display boundary, including historical records.
// Do not apply this to user comments, document names or API error details.
const systemLabels: Record<string, string> = {
  ...bankStatusLabel, ...auditActionLabel, ...reasonLabel, ...ruleLabel, ...checkCodeLabel, ...purposeLabel, ...companyStatusLabel,
  SME_CASHFLOW_V2: "Кредит по денежному потоку",
  SME_PROFILE_V2: "Кредит для ИП и ООО",
};

export function systemMessage(text: string): string {
  let display = text;
  for (const [name, label] of Object.entries(productNames)) display = display.replaceAll(name, label);
  return display
    .replaceAll("Деньги выданы (демо)", "Деньги выданы")
    .replaceAll("Демо-выдача после подписания договора", "Выдача после подписания договора")
    .replaceAll("Демо-выдача после подписания (симуляция)", "Выдача после подписания договора")
    .replaceAll("ContractSigned: демо-подпись", "Договор подписан")
    .replaceAll("Заявка получена из client.events", "Заявка получена")
    .replaceAll("ApplicationSubmitted получено из client.events", "Заявка получена")
    .replaceAll("GetCompanyProfile: OK; GetCreditHistory: OK", "Получены сведения о компании и кредитной истории")
    .replaceAll("Получены ответы ЕГРЮЛ/ФНС и БКИ (моки)", "Получены сведения о компании и кредитной истории")
    .replaceAll("Профиль не поддерживается текущим демо-конвейером", "Профиль требует проверки специалистом")
    .replaceAll("InfoProvided: клиент ответил на запрос", "Клиент ответил на запрос")
    .replaceAll("OfferAccepted: клиент принял оферту", "Клиент принял предложение")
    .replaceAll("OfferDeclined", "Клиент отказался от предложения")
    .replaceAll("ApplicationCancelled", "Клиент отозвал заявку")
    .replaceAll("Грейд скоринга", "Категория риска")
    .replace(/\bScore (\d+), grade ([A-E])\b/g, "Оценка риска: $1, категория $2")
    .replace(/\b[Gg]rade ([A-E])\b/g, "Категория $1")
    .replace(/\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*\b/g, (code) => systemLabels[code] ?? code);
}
