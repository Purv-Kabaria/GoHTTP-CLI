package reassembly

import (
	"encoding/binary"
	"io"
)

func ExtractSNI(r io.Reader) (string, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(r, header); err != nil {
		return "", err
	}

	if header[0] != 0x16 {
		return "", io.EOF
	}

	recordLen := int(binary.BigEndian.Uint16(header[3:5]))
	if recordLen > 8192 {
		recordLen = 8192
	}

	payload := make([]byte, recordLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return "", err
	}

	if len(payload) < 43 || payload[0] != 0x01 {
		return "", io.EOF
	}

	sessionIDLen := int(payload[38])
	offset := 39 + sessionIDLen

	if len(payload) < offset+2 {
		return "", io.EOF
	}
	cipherSuitesLen := int(binary.BigEndian.Uint16(payload[offset : offset+2]))
	offset += 2 + cipherSuitesLen

	if len(payload) < offset+1 {
		return "", io.EOF
	}
	compressionMethodsLen := int(payload[offset])
	offset += 1 + compressionMethodsLen

	if len(payload) < offset+2 {
		return "", io.EOF
	}
	extensionsLen := int(binary.BigEndian.Uint16(payload[offset : offset+2]))
	offset += 2

	end := offset + extensionsLen
	if end > len(payload) {
		end = len(payload)
	}

	for offset+4 <= end {
		extType := binary.BigEndian.Uint16(payload[offset : offset+2])
		extLen := int(binary.BigEndian.Uint16(payload[offset+2 : offset+4]))
		offset += 4

		if extType == 0x00 {
			if offset+5 <= end && extLen >= 5 {
				nameLen := int(binary.BigEndian.Uint16(payload[offset+3 : offset+5]))
				if offset+5+nameLen <= end {
					return string(payload[offset+5 : offset+5+nameLen]), nil
				}
			}
		}
		offset += extLen
	}

	return "", io.EOF
}