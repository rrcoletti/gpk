// gpk - read-only TUI for GitHub Projects v2 kanban boards
// Copyright (C) 2026  Rafael Coletti
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
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
