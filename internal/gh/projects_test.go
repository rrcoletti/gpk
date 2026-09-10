package gh

import "testing"

func TestStatusField(t *testing.T) {
	single := func(name string) FieldDef {
		return FieldDef{ID: "f_" + name, Name: name, Options: []SelectOption{{Name: "a"}}}
	}
	plain := func(name string) FieldDef {
		return FieldDef{ID: "f_" + name, Name: name}
	}

	tests := []struct {
		name     string
		fields   []FieldDef
		wantName string
		wantOK   bool
	}{
		{"status first", []FieldDef{single("Status"), single("Priority")}, "Status", true},
		{"status later, wins over first single", []FieldDef{single("Priority"), single("status")}, "status", true},
		{"falls back to first single-select", []FieldDef{plain("Text"), single("Priority")}, "Priority", true},
		{"ignores non-select fields", []FieldDef{plain("Status")}, "", false},
		{"empty", nil, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := StatusField(tt.fields)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got.Name != tt.wantName {
				t.Errorf("got %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}
