import type { PurposeTag } from "../api/types";

// Line icons for the four purposes; decorative, the text next to them carries the meaning.
export function PurposeIcon({ purpose }: { purpose: PurposeTag }) {
  return (
    <svg className="purpose-icon" viewBox="0 0 32 32" width="32" height="32" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      {purpose === "WORKING_CAPITAL" && (
        <>
          <path d="M6 12a10 10 0 0 1 17.5-5.5L26 9" />
          <path d="M26 3v6h-6" />
          <path d="M26 20a10 10 0 0 1-17.5 5.5L6 23" />
          <path d="M6 29v-6h6" />
          <path d="M16 11v10M13 13.5c0-1.4 1.3-2.5 3-2.5s3 1.1 3 2.5-1.3 2.5-3 2.5-3 1.1-3 2.5 1.3 2.5 3 2.5 3-1.1 3-2.5" />
        </>
      )}
      {purpose === "EQUIPMENT" && (
        <>
          <circle cx="16" cy="16" r="4" />
          <path d="M16 3v4M16 25v4M3 16h4M25 16h4M6.8 6.8l2.8 2.8M22.4 22.4l2.8 2.8M6.8 25.2l2.8-2.8M22.4 9.6l2.8-2.8" />
        </>
      )}
      {purpose === "OTHER" && (
        <>
          <rect x="4" y="4" width="10" height="10" rx="2" />
          <rect x="18" y="4" width="10" height="10" rx="2" />
          <rect x="4" y="18" width="10" height="10" rx="2" />
          <path d="M23 18v10M18 23h10" />
        </>
      )}
      {purpose === "EXPANSION" && (
        <>
          <path d="M4 27h24" />
          <path d="M7 27V17h6v10M15 27V11h6v16M23 27V6h4" />
          <path d="M20 5l4-3 4 3" />
        </>
      )}
    </svg>
  );
}
