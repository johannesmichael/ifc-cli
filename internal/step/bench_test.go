package step

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// generateSTEP builds a synthetic STEP file with n entities.
func generateSTEP(n int) string {
	var b strings.Builder
	b.WriteString("ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('bench'),'2;1');\nENDSEC;\nDATA;\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "#%d=IFCWALL('name%d',.ELEMENT.,#%d,(#%d,#%d),$,%d,3.14);\n",
			i, i, (i%n)+1, (i%n)+1, ((i+1)%n)+1, i+40)
	}
	b.WriteString("ENDSEC;\nEND-ISO-10303-21;\n")
	return b.String()
}

func BenchmarkLexer(b *testing.B) {
	data := []byte(generateSTEP(10000))
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lex := NewLexer(data)
		for {
			tok, err := lex.NextToken()
			if err != nil {
				b.Fatal(err)
			}
			if tok.Kind == TokenEOF {
				break
			}
		}
	}
}

func BenchmarkParser(b *testing.B) {
	data := []byte(generateSTEP(10000))
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := NewParser(data)
		for {
			_, err := p.Next()
			if err != nil {
				break
			}
		}
	}
}

func BenchmarkParseAll(b *testing.B) {
	data := []byte(generateSTEP(10000))
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseAll(data)
	}
}

func BenchmarkParseRealFile(b *testing.B) {
	data, err := os.ReadFile("../../ifc/BF1_ARCH_HA.ifc")
	if err != nil {
		b.Skip("no real IFC file available")
	}
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseAll(data)
	}
}
