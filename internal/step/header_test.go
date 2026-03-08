package step

import (
	"io"
	"testing"
)

func TestHeaderFullMetadata(t *testing.T) {
	src := []byte(`ISO-10303-21;
HEADER;
FILE_DESCRIPTION(('ViewDefinition [CoordinationView]'),'2;1');
FILE_NAME('model.ifc','2024-01-15T10:30:00',('John Doe','Jane Smith'),('ACME Corp'),'PreprocessorX','OriginApp','Auth123');
FILE_SCHEMA(('IFC4'));
ENDSEC;
DATA;
#1 = IFCPROJECT('guid',$,$,$,$,$,$,$,$);
ENDSEC;
END-ISO-10303-21;
`)
	p := NewParser(src)

	// Consume all entities to trigger header parsing
	for {
		_, err := p.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	meta := p.Metadata()
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}

	if meta.Description != "ViewDefinition [CoordinationView]" {
		t.Errorf("Description = %q, want %q", meta.Description, "ViewDefinition [CoordinationView]")
	}
	if meta.ImplementationLevel != "2;1" {
		t.Errorf("ImplementationLevel = %q, want %q", meta.ImplementationLevel, "2;1")
	}
	if meta.FileName != "model.ifc" {
		t.Errorf("FileName = %q, want %q", meta.FileName, "model.ifc")
	}
	if meta.Timestamp != "2024-01-15T10:30:00" {
		t.Errorf("Timestamp = %q, want %q", meta.Timestamp, "2024-01-15T10:30:00")
	}
	if len(meta.Author) != 2 || meta.Author[0] != "John Doe" || meta.Author[1] != "Jane Smith" {
		t.Errorf("Author = %v, want [John Doe, Jane Smith]", meta.Author)
	}
	if len(meta.Organization) != 1 || meta.Organization[0] != "ACME Corp" {
		t.Errorf("Organization = %v, want [ACME Corp]", meta.Organization)
	}
	if meta.Preprocessor != "PreprocessorX" {
		t.Errorf("Preprocessor = %q, want %q", meta.Preprocessor, "PreprocessorX")
	}
	if meta.OriginatingSystem != "OriginApp" {
		t.Errorf("OriginatingSystem = %q, want %q", meta.OriginatingSystem, "OriginApp")
	}
	if meta.Authorization != "Auth123" {
		t.Errorf("Authorization = %q, want %q", meta.Authorization, "Auth123")
	}
	if len(meta.SchemaIdentifiers) != 1 || meta.SchemaIdentifiers[0] != "IFC4" {
		t.Errorf("SchemaIdentifiers = %v, want [IFC4]", meta.SchemaIdentifiers)
	}
}

func TestHeaderMinimalMetadata(t *testing.T) {
	src := []byte(`ISO-10303-21;
HEADER;
FILE_DESCRIPTION((),'2;1');
FILE_NAME('','',(),$,$,$,$);
FILE_SCHEMA(('IFC2X3'));
ENDSEC;
DATA;
ENDSEC;
END-ISO-10303-21;
`)
	p := NewParser(src)

	_, err := p.Next()
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}

	meta := p.Metadata()
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}

	if meta.Description != "" {
		t.Errorf("Description = %q, want empty", meta.Description)
	}
	if meta.ImplementationLevel != "2;1" {
		t.Errorf("ImplementationLevel = %q, want %q", meta.ImplementationLevel, "2;1")
	}
	if meta.FileName != "" {
		t.Errorf("FileName = %q, want empty", meta.FileName)
	}
	if meta.Timestamp != "" {
		t.Errorf("Timestamp = %q, want empty", meta.Timestamp)
	}
	if len(meta.Author) != 0 {
		t.Errorf("Author = %v, want empty", meta.Author)
	}
	if meta.Preprocessor != "" {
		t.Errorf("Preprocessor = %q, want empty", meta.Preprocessor)
	}
	if len(meta.SchemaIdentifiers) != 1 || meta.SchemaIdentifiers[0] != "IFC2X3" {
		t.Errorf("SchemaIdentifiers = %v, want [IFC2X3]", meta.SchemaIdentifiers)
	}
}

func TestHeaderMultipleDescriptions(t *testing.T) {
	src := []byte(`ISO-10303-21;
HEADER;
FILE_DESCRIPTION(('ViewDefinition [CoordinationView]','ExchangeRequirement [Architecture]'),'2;1');
FILE_SCHEMA(('IFC4','IFC4X3'));
ENDSEC;
DATA;
ENDSEC;
END-ISO-10303-21;
`)
	p := NewParser(src)

	_, err := p.Next()
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}

	meta := p.Metadata()
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}

	if meta.Description != "ViewDefinition [CoordinationView]; ExchangeRequirement [Architecture]" {
		t.Errorf("Description = %q", meta.Description)
	}
	if len(meta.SchemaIdentifiers) != 2 || meta.SchemaIdentifiers[0] != "IFC4" || meta.SchemaIdentifiers[1] != "IFC4X3" {
		t.Errorf("SchemaIdentifiers = %v, want [IFC4, IFC4X3]", meta.SchemaIdentifiers)
	}
}

func TestHeaderNoHeaderSection(t *testing.T) {
	src := []byte(`#1 = IFCWALL('a');`)
	p := NewParser(src)

	ent, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ent.ID != 1 {
		t.Errorf("expected ID 1, got %d", ent.ID)
	}

	if p.Metadata() != nil {
		t.Error("expected nil metadata for file without header")
	}
}

func TestHeaderMetadataFromTestdata(t *testing.T) {
	src := mustReadTestdata(t, "minimal.ifc")
	p := NewParser(src)

	for {
		_, err := p.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	meta := p.Metadata()
	if meta == nil {
		t.Fatal("expected non-nil metadata from minimal.ifc")
	}

	if len(meta.SchemaIdentifiers) == 0 {
		t.Error("expected at least one schema identifier")
	}
}
