// Package brazil names the shapes Brazilian forms ask for: CPF, CNPJ, phone,
// postcode, licence plate and the real.
//
// They are shapes and not rules, and the distinction is the one hesape/mask
// draws: eleven digits are not a CPF until the check digits agree, and this
// package checks nothing. A field that takes a CPF validates it with the
// application's own rule, on the raw value [Pattern.Unmask] gives back.
//
// They are in a package of their own because the framework has no opinion about
// which countries exist -- see the doc of package mask -- and an application
// that is not Brazilian should not find them beside the generic ones.
package brazil

import "github.com/arandu-io/ayra/mask"

const (
	// CPF is the personal tax number: 123.456.789-09.
	CPF mask.Pattern = "000.000.000-00"
	// CNPJ is the company tax number: 12.345.678/0001-95.
	CNPJ mask.Pattern = "00.000.000/0000-00"
	// CEP is the postcode: 01310-100.
	CEP mask.Pattern = "00000-000"
)

// Document is a field that takes a CPF or a CNPJ: it is a CPF until the twelfth
// digit is typed.
var Document = mask.Alternatives{CPF, CNPJ}

// Phone is a landline of ten digits or a mobile of eleven: (11) 98765-4321. The
// field grows into the mobile shape at the eleventh digit.
var Phone = mask.Alternatives{"(00) 0000-0000", "(00) 00000-0000"}

// Plate is a licence plate, the old shape (ABC-1234) and the Mercosul one
// (ABC1D23) alike: three letters, a digit, a letter or digit, and two digits.
// The old plate is typed without the hyphen: ABC1234 is the plate, and the
// hyphen is only how it is printed. The letters are written in capitals whatever
// was typed, which is how a plate is printed and stored.
const Plate mask.Pattern = "UUU0X00"

// Real is the Brazilian currency: R$ 1.234,56, stored as 123456 centavos.
var Real = mask.Money{Symbol: "R$", Decimals: 2, Thousands: ".", Decimal: ","}
