// Package csvsafe neutralises the CSV cells that a spreadsheet would run as
// a formula (docs/specs/190-backend-defects.md B8).
package csvsafe

// Record returns record with every cell made safe by Cell. It changes record
// in place and returns it.
func Record(record []string) []string {
	for index, cell := range record {
		record[index] = Cell(cell)
	}

	return record
}

// Cell prefixes with a quote a cell that starts like a formula: =, +, -, @,
// or a tab or carriage return that a spreadsheet would skip before one. A
// learner's name such as "=HYPERLINK(...)" is then shown as text, not run
// when a manager opens the export.
func Cell(cell string) string {
	if cell == "" {
		return cell
	}

	switch cell[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + cell
	}

	return cell
}
