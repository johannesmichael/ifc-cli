package step

import "testing"

func FuzzLexer(f *testing.F) {
	f.Add([]byte("#1=IFCWALL('test');"))
	f.Add([]byte("$"))
	f.Add([]byte("'hello'"))
	f.Add([]byte(".ELEMENT."))
	f.Add([]byte("3.14E-5"))
	f.Add([]byte("/* comment */"))
	f.Add([]byte("#999=IFCPROJECT('guid',#2,$,.T.,(#3,#4),*,IFCLENGTHMEASURE(2.5));"))
	f.Add([]byte(""))
	f.Add([]byte("((((1))))"))

	f.Fuzz(func(t *testing.T, data []byte) {
		l := NewLexer(data)
		for i := 0; i < 10000; i++ {
			tok, err := l.NextToken()
			if err != nil {
				return
			}
			if tok.Kind == TokenEOF {
				return
			}
		}
	})
}
