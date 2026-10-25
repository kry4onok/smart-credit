package document

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/gabriel-vasile/mimetype"
)

// validateFile checks the file format without extracting financial data or
// modifying the original bytes. It does not verify the document's authenticity.
func validateFile(name string, content []byte) error {
	var valid bool
	switch strings.ToLower(filepath.Ext(name)) {
	case ".txt":
		valid = validText(content)
	case ".csv":
		valid = validText(content) && validCSV(content)
	case ".pdf":
		valid = bytes.HasPrefix(content, []byte("%PDF-")) &&
			bytes.Contains(content[max(0, len(content)-1024):], []byte("%%EOF"))
	case ".png":
		_, err := png.DecodeConfig(bytes.NewReader(content))
		valid = err == nil
	case ".jpg", ".jpeg":
		_, err := jpeg.DecodeConfig(bytes.NewReader(content))
		valid = err == nil
	case ".docx":
		valid = validOfficeArchive(content, "word/document.xml")
	case ".xlsx":
		valid = validOfficeArchive(content, "xl/workbook.xml")
	case ".doc":
		valid = mimetype.Detect(content).Is("application/msword")
	case ".xls":
		valid = mimetype.Detect(content).Is("application/vnd.ms-excel")
	default:
		return domain.InvalidFields(domain.ErrInvalidDocumentUpload,
			"Формат файла не поддерживается. Используйте PDF, PNG, JPG/JPEG, DOC/DOCX, XLS/XLSX, CSV или TXT.")
	}
	if !valid {
		return domain.InvalidFields(domain.ErrInvalidDocumentUpload,
			"Содержимое файла не соответствует расширению или формат не распознан. TXT и CSV должны быть в UTF-8; CSV — таблицей с разделителем запятая, точка с запятой или табуляция.")
	}
	return nil
}

func validText(content []byte) bool {
	if !utf8.Valid(content) || len(bytes.TrimSpace(bytes.TrimPrefix(content, []byte{0xef, 0xbb, 0xbf}))) == 0 {
		return false
	}
	for _, char := range string(content) {
		if unicode.IsControl(char) && char != '\t' && char != '\n' && char != '\r' {
			return false
		}
	}
	return true
}

func validCSV(content []byte) bool {
	content = bytes.TrimPrefix(content, []byte{0xef, 0xbb, 0xbf})
	for _, separator := range []rune{',', ';', '\t'} {
		reader := csv.NewReader(bytes.NewReader(content))
		reader.Comma, reader.ReuseRecord = separator, true
		header, err := reader.Read()
		if err != nil || len(header) < 2 {
			continue
		}
		for {
			_, err = reader.Read()
			if err != nil {
				break
			}
		}
		if err == io.EOF {
			return true
		}
	}
	return false
}

func validOfficeArchive(content []byte, mainPart string) bool {
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return false
	}
	// Inspect ZIP metadata only. Never extract an untrusted archive into the
	// filesystem or decompress arbitrary entries while accepting an upload.
	var hasTypes, hasDocument bool
	for _, file := range archive.File {
		if file.UncompressedSize64 == 0 {
			continue
		}
		switch file.Name {
		case "[Content_Types].xml":
			hasTypes = true
		case mainPart:
			hasDocument = true
		}
	}
	return hasTypes && hasDocument
}
