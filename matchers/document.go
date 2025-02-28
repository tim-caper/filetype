package matchers

import (
	"bytes"
	"encoding/binary"
)

var (
	TypeDoc  = newType("doc", "application/msword")
	TypeDocx = newType("docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	TypeXls  = newType("xls", "application/vnd.ms-excel")
	TypeXlsx = newType("xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	TypePpt  = newType("ppt", "application/vnd.ms-powerpoint")
	TypePptx = newType("pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation")
	TypeOdp  = newType("odp", "application/vnd.oasis.opendocument.presentation")
	TypeOds  = newType("ods", "application/vnd.oasis.opendocument.spreadsheet")
	TypeOdt  = newType("odt", "application/vnd.oasis.opendocument.text")
)

var Document = Map{
	TypeDoc:  Doc,
	TypeDocx: Docx,
	TypeXls:  Xls,
	TypeXlsx: Xlsx,
	TypePpt:  Ppt,
	TypePptx: Pptx,
	TypeOdp:  Odp,
	TypeOds:  Ods,
	TypeOdt:  Odt,
}

type docType int

const (
	TYPE_DOC docType = iota
	TYPE_DOCX
	TYPE_XLS
	TYPE_XLSX
	TYPE_PPT
	TYPE_PPTX
	TYPE_OOXML
	TYPE_ODP
	TYPE_ODS
	TYPE_ODT
)

//reference: https://bz.apache.org/ooo/show_bug.cgi?id=111457
func Doc(buf []byte) bool {
	if len(buf) > 513 {
		return buf[0] == 0xD0 && buf[1] == 0xCF &&
			buf[2] == 0x11 && buf[3] == 0xE0 &&
			buf[512] == 0xEC && buf[513] == 0xA5
	} else {
		return len(buf) > 3 &&
			buf[0] == 0xD0 && buf[1] == 0xCF &&
			buf[2] == 0x11 && buf[3] == 0xE0
	}
}

func Docx(buf []byte) bool {
	typ, ok := msooxml(buf)
	return ok && typ == TYPE_DOCX
}

func Xls(buf []byte) bool {
	if len(buf) > 513 {
		return buf[0] == 0xD0 && buf[1] == 0xCF &&
			buf[2] == 0x11 && buf[3] == 0xE0 &&
			buf[512] == 0x09 && buf[513] == 0x08
	} else {
		return len(buf) > 3 &&
			buf[0] == 0xD0 && buf[1] == 0xCF &&
			buf[2] == 0x11 && buf[3] == 0xE0
	}
}

func Xlsx(buf []byte) bool {
	typ, ok := msooxml(buf)
	return ok && typ == TYPE_XLSX
}

func Ppt(buf []byte) bool {
	if len(buf) > 513 {
		return buf[0] == 0xD0 && buf[1] == 0xCF &&
			buf[2] == 0x11 && buf[3] == 0xE0 &&
			buf[512] == 0xA0 && buf[513] == 0x46
	} else {
		return len(buf) > 3 &&
			buf[0] == 0xD0 && buf[1] == 0xCF &&
			buf[2] == 0x11 && buf[3] == 0xE0
	}
}

func Pptx(buf []byte) bool {
	typ, ok := msooxml(buf)
	return ok && typ == TYPE_PPTX
}

func msooxml(buf []byte) (typ docType, found bool) {
	signature := []byte{'P', 'K', 0x03, 0x04}

	// start by checking for ZIP local file header signature
	if ok := compareBytes(buf, signature, 0); !ok {
		return
	}

	// make sure the first file is correct
	if v, ok := checkMSOoml(buf, 0x1E); ok {
		return v, ok
	}

	if !compareBytes(buf, []byte("[Content_Types].xml"), 0x1E) &&
		!compareBytes(buf, []byte("_rels/.rels"), 0x1E) &&
		!compareBytes(buf, []byte("docProps"), 0x1E) {
		return
	}

	// skip to the second local file header
	// since some documents include a 520-byte extra field following the file
	// header, we need to scan for the next header
	startOffset := int(binary.LittleEndian.Uint32(buf[18:22]) + 49)
	idx := search(buf, startOffset, 6000)
	if idx == -1 {
		return
	}

	// now skip to the *third* local file header; again, we need to scan due to a
	// 520-byte extra field following the file header
	startOffset += idx + 4 + 26
	idx = search(buf, startOffset, 6000)
	if idx == -1 {
		return
	}

	// and check the subdirectory name to determine which type of OOXML
	// file we have.  Correct the mimetype with the registered ones:
	// http://technet.microsoft.com/en-us/library/cc179224.aspx
	startOffset += idx + 4 + 26
	if typ, ok := checkMSOoml(buf, startOffset); ok {
		return typ, ok
	}

	// OpenOffice/Libreoffice orders ZIP entry differently, so check the 4th file
	startOffset += 26
	idx = search(buf, startOffset, 6000)
	if idx == -1 {
		return TYPE_OOXML, true
	}

	startOffset += idx + 4 + 26
	if typ, ok := checkMSOoml(buf, startOffset); ok {
		return typ, ok
	} else {
		return TYPE_OOXML, true
	}
}

func compareBytes(slice, subSlice []byte, startOffset int) bool {
	sl := len(subSlice)

	if startOffset+sl > len(slice) {
		return false
	}

	s := slice[startOffset : startOffset+sl]
	for i := range s {
		if subSlice[i] != s[i] {
			return false
		}
	}

	return true
}

func checkMSOoml(buf []byte, offset int) (typ docType, ok bool) {
	ok = true

	switch {
	case compareBytes(buf, []byte("word/"), offset):
		typ = TYPE_DOCX
	case compareBytes(buf, []byte("ppt/"), offset):
		typ = TYPE_PPTX
	case compareBytes(buf, []byte("xl/"), offset):
		typ = TYPE_XLSX
	default:
		ok = false
	}

	return
}

func search(buf []byte, start, rangeNum int) int {
	length := len(buf)
	end := start + rangeNum
	signature := []byte{'P', 'K', 0x03, 0x04}

	if end > length {
		end = length
	}

	if start >= end {
		return -1
	}

	return bytes.Index(buf[start:end], signature)
}

func Odp(buf []byte) bool {
	return checkOdf(buf, TypeOdp.MIME.Value)
}

func Ods(buf []byte) bool {
	return checkOdf(buf, TypeOds.MIME.Value)
}

func Odt(buf []byte) bool {
	return checkOdf(buf, TypeOdt.MIME.Value)
}

// https://en.wikipedia.org/wiki/OpenDocument_technical_specification
// https://en.wikipedia.org/wiki/ZIP_(file_format)
func checkOdf(buf []byte, mimetype string) bool {
	if 38+len(mimetype) >= len(buf) {
		return false
	}
	// Perform all byte checks first for better performance
	// Check ZIP start
	if buf[0] != 'P' || buf[1] != 'K' || buf[2] != 3 || buf[3] != 4 {
		return false
	}
	// Now check the first file data
	// Compression method: not compressed
	if buf[8] != 0 || buf[9] != 0 {
		return false
	}
	// Filename length must be 8 for "mimetype"
	if buf[26] != 8 || buf[27] != 0 {
		return false
	}
	// Check the file contents sizes
	if int(buf[18]) != len(mimetype) ||
		buf[19] != 0 || buf[20] != 0 || buf[21] != 0 ||
		int(buf[22]) != len(mimetype) ||
		buf[23] != 0 || buf[24] != 0 || buf[25] != 0 {
		return false
	}
	// No extra field (for data offset below)
	if buf[28] != 0 || buf[29] != 0 {
		return false
	}
	// Finally check the file name and contents
	return string(buf[30:38]) == "mimetype" &&
		string(buf[38:38+len(mimetype)]) == mimetype
}
