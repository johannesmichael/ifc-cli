package step

// FileMetadata holds metadata extracted from the STEP file header section.
type FileMetadata struct {
	Description       string
	ImplementationLevel string
	FileName          string
	Timestamp         string
	Author            []string
	Organization      []string
	Preprocessor      string
	OriginatingSystem string
	Authorization     string
	SchemaIdentifiers []string
}

// parseHeaderEntity extracts metadata from a header entity (FILE_DESCRIPTION, FILE_NAME, FILE_SCHEMA).
func (p *Parser) parseHeaderEntity() error {
	typeName := p.cur.Value
	if err := p.advance(); err != nil {
		return err
	}

	if _, err := p.expect(TokenLParen); err != nil {
		return err
	}

	attrs, err := p.parseAttrList()
	if err != nil {
		return err
	}

	if _, err := p.expect(TokenRParen); err != nil {
		return err
	}

	if p.cur.Kind == TokenSemicolon {
		if err := p.advance(); err != nil {
			return err
		}
	}

	switch typeName {
	case "FILE_DESCRIPTION":
		p.extractFileDescription(attrs)
	case "FILE_NAME":
		p.extractFileName(attrs)
	case "FILE_SCHEMA":
		p.extractFileSchema(attrs)
	}

	return nil
}

func (p *Parser) extractFileDescription(attrs []StepValue) {
	if p.meta == nil {
		p.meta = &FileMetadata{}
	}
	if len(attrs) >= 1 {
		p.meta.Description = flattenStringList(attrs[0])
	}
	if len(attrs) >= 2 && attrs[1].Kind == KindString {
		p.meta.ImplementationLevel = attrs[1].Str
	}
}

func (p *Parser) extractFileName(attrs []StepValue) {
	if p.meta == nil {
		p.meta = &FileMetadata{}
	}
	if len(attrs) >= 1 && attrs[0].Kind == KindString {
		p.meta.FileName = attrs[0].Str
	}
	if len(attrs) >= 2 && attrs[1].Kind == KindString {
		p.meta.Timestamp = attrs[1].Str
	}
	if len(attrs) >= 3 {
		p.meta.Author = collectStrings(attrs[2])
	}
	if len(attrs) >= 4 {
		p.meta.Organization = collectStrings(attrs[3])
	}
	if len(attrs) >= 5 && attrs[4].Kind == KindString {
		p.meta.Preprocessor = attrs[4].Str
	}
	if len(attrs) >= 6 && attrs[5].Kind == KindString {
		p.meta.OriginatingSystem = attrs[5].Str
	}
	if len(attrs) >= 7 && attrs[6].Kind == KindString {
		p.meta.Authorization = attrs[6].Str
	}
}

func (p *Parser) extractFileSchema(attrs []StepValue) {
	if p.meta == nil {
		p.meta = &FileMetadata{}
	}
	if len(attrs) >= 1 {
		p.meta.SchemaIdentifiers = collectStrings(attrs[0])
	}
}

// flattenStringList extracts a display string from a StepValue that may be a string or a list of strings.
func flattenStringList(v StepValue) string {
	if v.Kind == KindString {
		return v.Str
	}
	if v.Kind == KindList && len(v.List) > 0 {
		// Join list items with "; " for display
		result := ""
		for i, item := range v.List {
			if item.Kind == KindString {
				if i > 0 {
					result += "; "
				}
				result += item.Str
			}
		}
		return result
	}
	return ""
}

// collectStrings extracts a []string from a StepValue that is a string or list of strings.
func collectStrings(v StepValue) []string {
	if v.Kind == KindString {
		return []string{v.Str}
	}
	if v.Kind == KindList {
		var result []string
		for _, item := range v.List {
			if item.Kind == KindString {
				result = append(result, item.Str)
			}
		}
		return result
	}
	return nil
}
