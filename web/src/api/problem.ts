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

function isProblem(value: unknown): value is Problem {
  return typeof value === "object" && value !== null && "status" in value && "title" in value;
}

export function toApiError(error: unknown, response?: Response): ApiError {
  if (error instanceof ApiError) return error;
  const status = response?.status ?? 0;
  if (isProblem(error)) {
    return new ApiError(status || error.status, error.detail || error.title || fallback[error.status] || "Ошибка", error.code, error.trace_id);
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
