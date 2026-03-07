package ifc

import (
	"testing"
)

func TestDecodeString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		// Basic cases
		{name: "empty", input: "", want: ""},
		{name: "plain ASCII", input: "Hello World", want: "Hello World"},

		// Backslash escape
		{name: "escaped backslash", input: `\\`, want: `\`},
		{name: "double escaped backslash", input: `\\\\`, want: `\\`},

		// Single quote escape
		{name: "escaped single quote", input: "''", want: "'"},
		{name: "text with escaped quote", input: "it''s", want: "it's"},

		// \S\ directive (ISO 8859 shift)
		{name: "S directive n to î", input: `\S\n`, want: "î"},   // 0x6E + 0x80 = 0xEE = î
		{name: "S directive h to è", input: `\S\h`, want: "è"},   // 0x68 + 0x80 = 0xE8 = è
		{name: "S directive q to ñ", input: `\S\q`, want: "ñ"},   // 0x71 + 0x80 = 0xF1 = ñ

		// \X\ directive (single hex byte)
		{name: "X directive E9 to é", input: `\X\E9`, want: "é"},
		{name: "X directive lowercase hex", input: `\X\e9`, want: "é"},
		{name: "X directive FC to ü", input: `\X\FC`, want: "ü"},

		// \X2\ directive (UCS-2)
		{name: "X2 single char é", input: `\X2\00E9\X0\`, want: "é"},
		{name: "X2 Hello", input: `\X2\00480065006C006C006F\X0\`, want: "Hello"},
		{name: "X2 Chinese chars", input: `\X2\4F60597D\X0\`, want: "你好"},

		// \X4\ directive (UCS-4)
		{name: "X4 emoji grinning face", input: `\X4\0001F600\X0\`, want: "\U0001F600"},
		{name: "X4 multiple codepoints", input: `\X4\0001F6000001F601\X0\`, want: "\U0001F600\U0001F601"},

		// Mixed directives
		{name: "mixed text and X2", input: `Hello \X2\00E9\X0\ World`, want: "Hello é World"},
		{name: "multiple directives", input: `\S\q \X\E9 \X2\00E9\X0\`, want: "ñ é é"},
		{name: "backslash then text", input: `path\\to\\file`, want: `path\to\file`},

		// Real-world IFC patterns
		{name: "German umlaut in wall name", input: `Au\X\DFenwand`, want: "Außenwand"},
		{name: "accented architect name", input: `Ren\X\E9 Dupont`, want: "René Dupont"},
		{name: "Japanese building name", input: `\X2\6771\X0\ Building`, want: "東 Building"},

		// Unrecognized escapes preserved
		{name: "unrecognized escape preserved", input: `\N\something`, want: `\N\something`},

		// Error cases
		{name: "X2 odd hex length", input: `\X2\00E\X0\`, wantErr: true},
		{name: "X4 odd hex length", input: `\X4\0001F6\X0\`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeString(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result: %q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("DecodeString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
