import type { Problem } from "./types";

// ApiError carries a message the interface can show as is, plus the machine code.
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
    readonly code?: string,
    readonly traceId?: string,
  ) {
    super(message);
  }
}

const fallback: Record<number, string> = {
  0: "Нет связи с сервером. Проверьте подключение и повторите.",
  400: "Запрос не принят: проверьте введённые данные.",
  401: "Сессия закончилась. Войдите снова.",
  403: "Это действие недоступно для вашей учётной записи.",
  404: "Запись не найдена или недоступна.",
  409: "Данные успели измениться. Обновите страницу и повторите действие.",
  422: "Не выполнено условие продукта. Проверьте данные заявки.",
  429: "Слишком много попыток. Подождите минуту.",
  501: "Эта операция ещё не подключена на сервере.",
  502: "Сервис временно недоступен. Повторите через минуту.",
  503: "Сервис временно недоступен. Повторите через минуту.",
};

// Newer handlers send only a machine code (title equals code, no detail); these are their texts.
const codeMessages: Record<string, string> = {
  CONTEXT_STALE: "Данные заявки обновились. Повторите запрос — помощник учтёт свежие сведения.",
  UNSUPPORTED_CATALOG_VERSION: "Версия помощника изменилась. Обновите страницу.",
  HELP_INCOMPATIBLE: "Помощник временно недоступен. Попробуйте позже.",
  INVALID_HELP_REQUEST: "Помощник не смог разобрать запрос.",
  APPLICATION_NOT_FOUND: "Заявка не найдена или недоступна.",
  ASSESSMENT_NOT_FOUND: "Оценка по заявке ещё не готова.",
  IF_MATCH_REQUIRED: "Данные успели измениться. Обновите страницу.",
  INVALID_IF_MATCH: "Данные успели измениться. Обновите страницу.",
  VERSION_CONFLICT: "Данные успели измениться. Обновите страницу и повторите действие.",
  APPLICATION_VERSION_CONFLICT: "Заявка уже изменена. Обновите карточку и повторите.",
  INVALID_ASSESSMENT_INPUT: "Проверьте введённые параметры: часть значений вне допустимого диапазона.",
  ASSESSMENT_INPUT_NOT_EDITABLE: "Параметры можно менять только в черновике или при запросе данных.",
  RECHECK_NOT_ALLOWED: "Повторную оценку можно запустить только по своему делу на ручном разборе.",
  CANDIDATE_NOT_FOUND: "Банк принимает только вариант из актуального расчёта. Выберите его в списке расчётных вариантов.",
  ASSESSMENT_STALE: "Оценка устарела. Пересчитайте её и выберите вариант заново.",
  ASSESSMENT_REQUIRED: "По делу ещё нет оценки денежного потока.",
  UNAUTHORIZED: "Сессия закончилась. Войдите снова.",
  FORBIDDEN: "Это действие недоступно для вашей учётной записи.",
};

function isProblem(value: unknown): value is Problem {
  return typeof value === "object" && value !== null && "status" in value && "title" in value;
}

export function toApiError(error: unknown, response?: Response): ApiError {
  if (error instanceof ApiError) return error;
  const status = response?.status ?? 0;
  if (isProblem(error)) {
    const codeOnly = !error.detail && error.title === error.code;
    // Some handlers wrap a domain code into the detail ("decide case: …: CANDIDATE_NOT_FOUND").
    const tail = error.detail?.match(/\b([A-Z][A-Z_]{3,})$/)?.[1];
    const known = tail ? codeMessages[tail] : undefined;
    const message = codeOnly ? (codeMessages[error.code] ?? fallback[error.status] ?? "Ошибка") : known || error.detail || error.title || fallback[error.status] || "Ошибка";
    return new ApiError(status || error.status, message, error.code, error.trace_id);
  }
  return new ApiError(status, fallback[status] ?? (status >= 500 ? fallback[503] : "Не удалось выполнить действие."));
}

type Result<T> = { data?: T; error?: unknown; response: Response };

// call unwraps an openapi-fetch result: data on success, ApiError otherwise.
export async function call<T>(request: Promise<Result<T>>): Promise<T> {
  let result: Result<T>;
  try {
    result = await request;
  } catch (error) {
    throw toApiError(error);
  }
  if (!result.response.ok) throw toApiError(result.error, result.response);
  return result.data as T;
}

export function errorMessage(error: unknown): string {
  return error instanceof ApiError ? error.message : toApiError(error).message;
}
