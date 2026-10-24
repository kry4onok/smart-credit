import { useEffect, useRef, useState } from "react";
import { clientApi } from "../api/http";
import { ApiError, call } from "../api/problem";
import type { DocumentMetadata, DocumentType } from "../api/types";
import { dateTime, parseRubles, rublesInput } from "../lib/format";
import { documentHint, documentLabel } from "../lib/labels";
import { uuid } from "../lib/util";
import { ErrorNote, useAction } from "./ui";

type MoneyInputProps = {
  id: string;
  value: number | undefined;
  onChange: (kopecks: number | null) => void;
  allowNegative?: boolean;
  disabled?: boolean;
  invalid?: boolean;
  "aria-describedby"?: string;
};

// MoneyInput edits rubles as text and reports kopecks; null means the text is not a valid amount.
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
          onChange(e.target.value.trim() === "" ? null : parseRubles(e.target.value, allowNegative));
        }}
      />
      <span aria-hidden>₽</span>
    </div>
  );
}

const MAX_FILE = 5 * 1024 * 1024;

type UploadProps = {
  applicationId: string;
  type: DocumentType;
  documents: DocumentMetadata[];
  enabled: boolean;
  onUploaded: (doc: DocumentMetadata) => void;
};

// DocumentSlot shows one required document type: what is attached and a button to attach a file.
export function DocumentSlot({ applicationId, type, documents, enabled, onUploaded }: UploadProps) {
  const input = useRef<HTMLInputElement>(null);
  const upload = useAction();
  const attached = documents.filter((d) => d.document_type === type);

  async function send(file: File) {
    if (file.size > MAX_FILE) {
      upload.setError(new ApiError(413, "Файл больше 5 МБ. Выберите файл поменьше."));
      return;
    }
    const doc = await upload.run(() =>
      call(
        clientApi.POST("/client/applications/{application_id}/documents", {
          params: { path: { application_id: applicationId }, header: { "Idempotency-Key": uuid() } },
          body: { document_type: type, file: file as unknown as string },
          bodySerializer: () => {
            const form = new FormData();
            form.append("document_type", type);
            form.append("file", file, file.name);
            return form;
          },
        }),
      ),
    );
    if (doc) onUploaded(doc);
  }

  return (
    <div className={`doc-slot${attached.length ? " doc-slot-done" : ""}`}>
      <div className="doc-slot-text">
        <strong>{documentLabel[type]}</strong>
        <span>{documentHint[type]}</span>
        {attached.map((d) => (
          <span key={d.id} className="doc-file">
            {d.file_name}, загружен {dateTime(d.uploaded_at)}
            {d.is_mock && <em className="tag">демо-документ</em>}
          </span>
        ))}
        {upload.error ? <ErrorNote error={upload.error} /> : null}
      </div>
      {enabled && (
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
