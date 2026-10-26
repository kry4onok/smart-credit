# Smart Credit demo statement TXT v1

This is an explicit parser contract for a **fictional demo format**. It is not a bank statement standard, proof of authenticity, or a source of annual revenue, accounting profit, or obligations. The source example is in `testdata/example.txt`.

The parser is not wired into document upload, validation, or assessment: no service imports it, uploaded files are only format-checked and stored, and no values are extracted from them. The free-form [`docs/demo-bank-statement.txt`](../../../../../docs/demo-bank-statement.txt) (a sample demo upload) does not follow this format.

`Parse([]byte)` accepts UTF-8 text up to 5 MiB. Blank lines and CRLF are allowed. The first nonblank line must be `ДЕМОНСТРАЦИОННАЯ ВЫПИСКА ПО СЧЁТУ`. Free-text introductory lines before `Тип документа:` and text after `ПРИМЕЧАНИЕ` carry no parsed data. Structured fields must have the exact Russian labels and order shown in the fixture:

1. `Тип документа: BANK_STATEMENT`, owner, account ID, period start/end as `YYYY-MM-DD`, `Валюта: RUB`.
2. `ОСТАТКИ И ОБОРОТЫ` and opening balance, total credits, total debits, closing balance.
3. `ОПЕРАЦИИ` and zero or more six-line operation blocks: ID, date, direction, amount, counterparty, purpose.

Amounts are nonnegative rubles with exactly two decimal digits and `.` as separator. Operations must have positive amounts, unique IDs, and dates inside the stated period. Directions are `Поступление` and `Списание`. The parser verifies operation sums against stated turnovers and `opening + credits - debits = closing`; arithmetic overflow is an error. It returns exact integer kopecks and dates, without estimated financial metrics.

Errors carry `ParseError.Code` and, for malformed fields, a source line: `UNSUPPORTED_FORMAT`, `INVALID_FIELD`, `INCONSISTENT_TOTALS`, or `LIMIT_EXCEEDED`. Other uploaded types such as PDF, images, DOCX, XLSX, and arbitrary TXT/CSV are unsupported by this parser.
