package document

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
	"unicode/utf16"
)

func TestExecuteValidatesFileFormat(t *testing.T) {
	var pngFile, jpegFile bytes.Buffer
	picture := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if err := png.Encode(&pngFile, picture); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegFile, picture, nil); err != nil {
		t.Fatal(err)
	}
	docx := officeArchive(t, "word/document.xml")
	xlsx := officeArchive(t, "xl/workbook.xml")
	for _, tc := range []struct {
		name    string
		file    string
		content []byte
		valid   bool
	}{
		{"UTF-8 text", "выписка.txt", []byte("Демо-выписка\r\nСумма\t1000.00"), true},
		{"uppercase extension", "ВЫПИСКА.TXT", []byte("statement"), true},
		{"UTF-8 BOM", "statement.txt", []byte("\xef\xbb\xbfДемо-выписка"), true},
		{"comma CSV", "statement.csv", []byte("date,amount\n2026-10-01,100.00\n"), true},
		{"semicolon CSV", "statement.csv", []byte("\xef\xbb\xbfdate;amount\r\n2026-10-01;100,00\r\n"), true},
		{"tab CSV", "statement.csv", []byte("date\tamount\n2026-10-01\t100.00\n"), true},
		{"quoted CSV", "statement.csv", []byte("date,description\n2026-10-01,\"delivery, paid\"\n"), true},
		{"PDF signature", "statement.pdf", []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n%%EOF\n"), true},
		{"PNG", "statement.png", pngFile.Bytes(), true},
		{"JPG", "statement.jpg", jpegFile.Bytes(), true},
		{"JPEG", "statement.jpeg", jpegFile.Bytes(), true},
		{"DOCX", "statement.docx", docx, true},
		{"XLSX", "statement.xlsx", xlsx, true},
		{"DOC container", "statement.doc", oleContainer("WordDocument"), true},
		{"XLS container", "statement.xls", oleContainer("Workbook"), true},
		{"executable", "statement.exe", []byte("MZ executable"), false},
		{"missing extension", "statement", []byte("statement"), false},
		{"double extension", "statement.pdf.exe", []byte("%PDF-1.4\n%%EOF"), false},
		{"renamed text", "statement.pdf", []byte("not a PDF"), false},
		{"truncated PDF", "statement.pdf", []byte("%PDF-1.4\n"), false},
		{"renamed image", "statement.jpeg", pngFile.Bytes(), false},
		{"truncated image", "statement.png", pngFile.Bytes()[:8], false},
		{"renamed Office", "statement.docx", xlsx, false},
		{"renamed legacy Office", "statement.xls", oleContainer("WordDocument"), false},
		{"generic ZIP", "statement.docx", officeArchive(t, "unrelated.txt"), false},
		{"truncated ZIP", "statement.xlsx", xlsx[:len(xlsx)/2], false},
		{"OLE signature alone", "statement.doc", []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, false},
		{"binary text", "statement.txt", []byte{'a', 0, 'b'}, false},
		{"late binary text", "statement.txt", append(bytes.Repeat([]byte{'a'}, 4096), 0xff), false},
		{"blank text", "statement.txt", []byte("\xef\xbb\xbf \r\n\t"), false},
		{"ragged CSV", "statement.csv", []byte("date,amount\n2026-10-01,100,extra\n"), false},
		{"broken quotes CSV", "statement.csv", []byte("date,amount\n2026-10-01,\"100\n"), false},
		{"plain text as CSV", "statement.csv", []byte("just some words"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := validParams()
			params.FileName, params.Content = tc.file, bytes.Clone(tc.content)
			repository := &repositoryMock{}
			_, err := New(repository).Execute(t.Context(), params)
			if tc.valid {
				if err != nil || !repository.called || !bytes.Equal(repository.params.Content, tc.content) || repository.params.FileName != tc.file {
					t.Fatalf("valid original did not reach storage: called=%v, err=%v", repository.called, err)
				}
				return
			}
			if err == nil || repository.called {
				t.Fatalf("invalid upload: called=%v, err=%v", repository.called, err)
			}
		})
	}
}

func officeArchive(t *testing.T, mainPart string) []byte {
	t.Helper()
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	for _, name := range []string{"[Content_Types].xml", mainPart} {
		file, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte("<root/>")); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

// Minimal OLE directory fixture for format identification, not a full document.
func oleContainer(streamName string) []byte {
	data := make([]byte, 1536)
	copy(data, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
	binary.LittleEndian.PutUint16(data[26:], 3)
	binary.LittleEndian.PutUint16(data[28:], 0xfffe)
	binary.LittleEndian.PutUint16(data[30:], 9)
	binary.LittleEndian.PutUint16(data[32:], 6)
	binary.LittleEndian.PutUint32(data[44:], 1)    // FAT sector count.
	binary.LittleEndian.PutUint32(data[48:], 0)    // Directory sector.
	binary.LittleEndian.PutUint32(data[56:], 4096) // Mini-stream cutoff.
	binary.LittleEndian.PutUint32(data[60:], 0xfffffffe)
	binary.LittleEndian.PutUint32(data[68:], 0xfffffffe)
	for offset := 76; offset < 512; offset += 4 {
		binary.LittleEndian.PutUint32(data[offset:], 0xffffffff)
	}
	binary.LittleEndian.PutUint32(data[76:], 1) // FAT lives in sector 1.
	entry := data[512:640]
	name := utf16.Encode([]rune(streamName + "\x00"))
	for i, char := range name {
		binary.LittleEndian.PutUint16(entry[i*2:], char)
	}
	binary.LittleEndian.PutUint16(entry[64:], uint16(len(name)*2))
	entry[66] = 2 // User stream.
	binary.LittleEndian.PutUint32(entry[116:], 0xfffffffe)
	binary.LittleEndian.PutUint32(data[1024:], 0xfffffffe) // End of directory chain.
	binary.LittleEndian.PutUint32(data[1028:], 0xfffffffd) // FAT sector.
	return data
}
