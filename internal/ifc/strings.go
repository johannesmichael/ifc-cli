package ifc

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"
)

// DecodeString decodes IFC/STEP encoded string directives into a UTF-8 string.
// It handles: \\ (literal backslash), \S\X (ISO 8859 shift), \X\HH (hex byte),
// \X2\HHHH...\X0\ (UCS-2), \X4\HHHHHHHH...\X0\ (UCS-4), and '' (literal quote).
func DecodeString(raw string) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.Grow(len(raw))

	i := 0
	for i < len(raw) {
		// Handle escaped single quote
		if raw[i] == '\'' && i+1 < len(raw) && raw[i+1] == '\'' {
			b.WriteByte('\'')
			i += 2
			continue
		}

		// Handle backslash directives
		if raw[i] != '\\' {
			b.WriteByte(raw[i])
			i++
			continue
		}

		// We have a backslash at position i
		if i+1 >= len(raw) {
			b.WriteByte('\\')
			i++
			continue
		}

		switch raw[i+1] {
		case '\\':
			// \\ → literal backslash
			b.WriteByte('\\')
			i += 2

		case 'S':
			// \S\X → ISO 8859 shift: set high bit on X
			if i+3 < len(raw) && raw[i+2] == '\\' {
				ch := raw[i+3]
				r := rune(ch) + 0x80
				b.WriteRune(r)
				i += 4
			} else {
				// Malformed: preserve as-is
				b.WriteByte('\\')
				i++
			}

		case 'X':
			if i+2 >= len(raw) {
				b.WriteByte('\\')
				i++
				continue
			}

			switch raw[i+2] {
			case '\\':
				// \X\HH → single hex byte, decoded as ISO 8859-1
				if i+4 < len(raw) {
					hexStr := raw[i+3 : i+5]
					bs, err := hex.DecodeString(hexStr)
					if err != nil {
						// Malformed hex: preserve as-is
						b.WriteByte('\\')
						i++
						continue
					}
					// ISO 8859-1 byte values map directly to Unicode code points
					b.WriteRune(rune(bs[0]))
					i += 5
				} else {
					b.WriteByte('\\')
					i++
				}

			case '2':
				// \X2\HHHH...\X0\ → UCS-2
				if i+3 >= len(raw) || raw[i+3] != '\\' {
					b.WriteByte('\\')
					i++
					continue
				}
				endIdx := strings.Index(raw[i+4:], `\X0\`)
				if endIdx < 0 {
					b.WriteByte('\\')
					i++
					continue
				}
				hexData := raw[i+4 : i+4+endIdx]
				if len(hexData)%4 != 0 {
					return "", fmt.Errorf("\\X2\\ hex data length %d is not a multiple of 4", len(hexData))
				}
				for j := 0; j < len(hexData); j += 4 {
					bs, err := hex.DecodeString(hexData[j : j+4])
					if err != nil {
						return "", fmt.Errorf("invalid hex in \\X2\\ at offset %d: %w", j, err)
					}
					cp := rune(uint16(bs[0])<<8 | uint16(bs[1]))
					b.WriteRune(cp)
				}
				i = i + 4 + endIdx + 4 // skip past \X0\

			case '4':
				// \X4\HHHHHHHH...\X0\ → UCS-4
				if i+3 >= len(raw) || raw[i+3] != '\\' {
					b.WriteByte('\\')
					i++
					continue
				}
				endIdx := strings.Index(raw[i+4:], `\X0\`)
				if endIdx < 0 {
					b.WriteByte('\\')
					i++
					continue
				}
				hexData := raw[i+4 : i+4+endIdx]
				if len(hexData)%8 != 0 {
					return "", fmt.Errorf("\\X4\\ hex data length %d is not a multiple of 8", len(hexData))
				}
				for j := 0; j < len(hexData); j += 8 {
					bs, err := hex.DecodeString(hexData[j : j+8])
					if err != nil {
						return "", fmt.Errorf("invalid hex in \\X4\\ at offset %d: %w", j, err)
					}
					cp := rune(uint32(bs[0])<<24 | uint32(bs[1])<<16 | uint32(bs[2])<<8 | uint32(bs[3]))
					if !utf8.ValidRune(cp) {
						return "", fmt.Errorf("invalid Unicode code point U+%04X in \\X4\\", cp)
					}
					b.WriteRune(cp)
				}
				i = i + 4 + endIdx + 4

			default:
				// Unrecognized \X followed by something else: preserve
				b.WriteByte('\\')
				i++
			}

		default:
			// Unrecognized escape: preserve as-is
			b.WriteByte('\\')
			i++
		}
	}

	return b.String(), nil
}
