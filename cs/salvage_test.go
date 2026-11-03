//go:build !wasi && !wasm

package cs

import "testing"

func TestSalvage_decay(t *testing.T) {
	tests := []struct {
		name  string
		Cargo Cargo
		want  Cargo
	}{
		{"no op, no cargo", Cargo{}, Cargo{}},
		{"decay each 8%", Cargo{Ironium: 100, Boranium: 100, Germanium: 100}, Cargo{Ironium: 92, Boranium: 92, Germanium: 92}},
		{"minimum eight per mineral", Cargo{Ironium: 20, Boranium: 10, Germanium: 9}, Cargo{Ironium: 12, Boranium: 2, Germanium: 1}},
		{"decay to 0", Cargo{Ironium: 2, Boranium: 2, Germanium: 2}, Cargo{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			salvage := &Salvage{
				Cargo: tt.Cargo,
			}
			salvage.decay(&rules)
			if salvage.Cargo != tt.want {
				t.Errorf("Salvage.decay() = %v, want %v", salvage.Cargo, tt.want)
			}

		})
	}
}
