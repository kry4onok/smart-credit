import { useEffect, useRef, useState } from "react";
import { clientApi } from "../api/http";
import { ApiError, call } from "../api/problem";
import type { DocumentMetadata, DocumentType, LegalForm, Requirement } from "../api/types";
import { date, dateTime, parseRubles, rublesInput } from "../lib/format";
import { documentSource } from "../lib/external";
import { documentHint, documentLabel } from "../lib/labels";
import { uuid } from "../lib/util";
import { ErrorNote, useAction } from "./ui";

type MoneyInputProps = {
  id: string;
  value: number | undefined;
  onChange: (kopecks: number | null | undefined) => void;
  allowNegative?: boolean;
  disabled?: boolean;
  invalid?: boolean;
  "aria-describedby"?: string;
};

// MoneyInput edits rubles as text and reports kopecks: undefined when the field is empty,
// null when the text is not a valid amount.
export function MoneyInput({ id, value, onChange, allowNegative, disabled, invalid, "aria-describedby": describedBy }: MoneyInputProps) {
  const [text, setText] = useState(rublesInput(value));
  const focused = useRef(false);
  useEffect(() => {
    if (!focused.current) setText(rublesInput(value));
  }, [value]);
  return (
    <div className="input-affix">
      <input
        id={id}
        inputMode="decimal"
        autoComplete="off"
        value={text}
        disabled={disabled}
        aria-invalid={invalid || undefined}
        aria-describedby={describedBy}
        onFocus={() => (focused.current = true)}
        onBlur={() => {
          focused.current = false;
          const parsed = parseRubles(text, allowNegative);
          if (parsed !== null) setText(rublesInput(parsed));
        }}
        onChange={(e) => {
          setText(e.target.value);
          onChange(e.target.value.trim() === "" ? undefined : parseRubles(e.target.value, allowNegative));
        }}
      />
      <span aria-hidden>₽</span>
    </div>
  );
}

const MAX_FILE = 5 * 1024 * 1024;

type UploadProps = {
  applicationId: string;
  // Legacy slot: one document type required by the product.
  type?: DocumentType;
  // Profile v2 slot: one requirement from the bank's plan, with its accepted types and period.
  requirement?: Requirement;
  legalForm?: LegalForm;
  documents: DocumentMetadata[];
  enabled: boolean;
  onUploaded: (doc: DocumentMetadata) => void;
};

function periodText(period: { from: string; to: string } | undefined): string | null {
  if (!period) return null;
  return period.from === period.to ? `на ${date(period.from)}` : `за ${date(period.from)} — ${date(period.to)}`;
}

// The backend reason says «за указанный период»; put the actual dates in its place.
function requirementText(r: Requirement): string {
  const period = periodText(r.period);
  if (!period) return r.reason;
  return r.reason.includes("за указанный период") ? r.reason.replace("за указанный период", period) : `${r.reason}, ${period}`;
}

// DocumentSlot shows one required document: what is attached, where to get it and a button to attach a file.
export function DocumentSlot({ applicationId, type, requirement, legalForm, documents, enabled, onUploaded }: UploadProps) {
  const input = useRef<HTMLInputElement>(null);
  const upload = useAction();
  const types = requirement ? requirement.accepted_document_types : type ? [type] : [];
  const [chosen, setChosen] = useState<DocumentType | undefined>(types[0]);
  const docType = chosen ?? types[0];
  const attached = requirement
    ? documents.filter((d) => d.requirement_id === requirement.id || requirement.document_ids.includes(d.id))
    : documents.filter((d) => d.document_type === type);
  const done = attached.length > 0 || requirement?.status === "PROVIDED";
  const source = docType ? documentSource(docType, legalForm) : null;

  async function send(file: File) {
    if (!docType) return;
    if (file.size === 0) {
      upload.setError(new ApiError(422, "Файл пустой. Выберите документ с содержимым."));
      return;
    }
    if (file.size > MAX_FILE) {
      upload.setError(new ApiError(413, "Файл больше 5 МБ. Выберите файл поменьше."));
      return;
    }
    const doc = await upload.run(() =>
      call(
        clientApi.POST("/client/applications/{application_id}/documents", {
          params: { path: { application_id: applicationId }, header: { "Idempotency-Key": uuid() } },
          body: { document_type: docType, file: file as unknown as string },
          bodySerializer: () => {
            const form = new FormData();
            form.append("document_type", docType);
            if (requirement) {
              form.append("requirement_id", requirement.id);
              if (requirement.period) {
                form.append("coverage_from", requirement.period.from);
                form.append("coverage_to", requirement.period.to);
              }
            }
            form.append("file", file, file.name);
            return form;
          },
        }),
      ),
    );
    if (doc) onUploaded(doc);
  }

  return (
    <div className={`doc-slot${done ? " doc-slot-done" : ""}`}>
      <div className="doc-slot-text">
        <strong>{requirement?.label ?? (type ? documentLabel[type] : "Документ")}</strong>
        <span>
          {requirement ? requirementText(requirement) : type ? documentHint[type] : null}
        </span>
        {types.length > 1 && enabled && (
          <label className="doc-type">
            Подойдёт:
            <select value={docType} onChange={(e) => setChosen(e.target.value as DocumentType)}>
              {types.map((t) => (
                <option key={t} value={t}>
                  {documentLabel[t]}
                </option>
              ))}
            </select>
          </label>
        )}
        {types.length === 1 && requirement && documentLabel[types[0]] !== requirement.label && <span>Подойдёт: {documentLabel[types[0]]}</span>}
        {source && !done && (
          <span className="doc-source">
            {source.how}
            {source.service && (
              <>
                {" "}
                <a href={source.service.url} target="_blank" rel="noreferrer noopener">
                  {source.service.label} ↗
                </a>
              </>
            )}
          </span>
        )}
        {attached.map((d) => (
          <span key={d.id} className="doc-file">
            {d.file_name}, загружен {dateTime(d.uploaded_at)}
            {d.is_mock && <em className="tag">демо-документ</em>}
          </span>
        ))}
        {attached.length > 0 && <span role="status">Файл сохранён. Данные из него не извлекаются — заполните поля формы вручную.</span>}
        {upload.error ? <ErrorNote error={upload.error} /> : null}
      </div>
      {enabled && docType && (
        <>
          <input
            ref={input}
            type="file"
            hidden
            accept=".pdf,.png,.jpg,.jpeg,.xlsx,.xls,.csv,.txt,.doc,.docx"
            onChange={(e) => {
              const file = e.target.files?.[0];
              e.target.value = "";
              if (file) void send(file);
            }}
          />
          <button type="button" className="btn btn-secondary btn-sm" disabled={upload.busy} onClick={() => input.current?.click()}>
            {upload.busy ? "Загружаем…" : attached.length ? "Заменить файл" : "Приложить файл"}
          </button>
        </>
      )}
    </div>
  );
}
