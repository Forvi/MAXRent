package pdf

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"

	"github.com/Forvi/maxrent/internal/domain/contract"
)

// Шрифты встроены в бинарь: кириллица в PDF не поддерживается без них,
// а полагаться на шрифты системы или сеть при сборке нельзя.
//
//go:embed fonts/DejaVuSans.ttf
var fontRegular []byte

//go:embed fonts/DejaVuSans-Bold.ttf
var fontBold []byte

// Параметры страницы и текста.
const (
	fontFamily    = "dejavu"
	pageFormat    = "A4"
	marginLeft    = 20.0
	marginTop     = 20.0
	lineHeight    = 5.2
	bodySize      = 10.0
	titleSize     = 14.0
	sectionSize   = 11.0
	titleGapAbove = 8.0
)

// Generator Формирует договор в формате PDF.
type Generator struct {
	// BodyFontFamily название встроенного шрифта.
	BodyFontFamily string
}

// NewGenerator Создаёт генератор договора.
func NewGenerator() *Generator {
	return &Generator{BodyFontFamily: fontFamily}
}

// Render Собирает договор и возвращает содержимое PDF-файла.
func (g *Generator) Render(d contract.Document) ([]byte, error) {
	if !d.IsReady() {
		return nil, fmt.Errorf("%w: не хватает данных: %s",
			contract.ErrNotReady, strings.Join(d.MissingFields(), ", "))
	}

	pdf := fpdf.NewCustom(&fpdf.InitType{
		UnitStr:    "mm",
		SizeStr:    pageFormat,
		FontDirStr: "",
	})

	pdf.SetMargins(marginLeft, marginTop, marginLeft)
	pdf.SetAutoPageBreak(true, marginTop)
	pdf.AddPage()

	// Кириллица работает только со встроенным шрифтом.
	pdf.AddUTF8FontFromBytes(fontFamily, "", fontRegular)
	pdf.AddUTF8FontFromBytes(fontFamily, "B", fontBold)

	g.writeSections(pdf, buildTemplate(d))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("write pdf: %w", err)
	}

	return buf.Bytes(), nil
}

// writeSections Выводит разделы и пункты в общем потоке:
// так заголовки не отрываются от своих пунктов при переносе страниц.
func (g *Generator) writeSections(pdf *fpdf.Fpdf, sections []section) {
	pdf.SetFont(fontFamily, "B", titleSize)
	pdf.MultiCell(0, lineHeight+1, strings.ToUpper(sections[0].title), "", "C", false)
	pdf.Ln(lineHeight)

	for i, s := range sections {
		// Заголовок первого раздела выведен выше как название документа.
		if s.title != "" && i > 0 {
			pdf.Ln(titleGapAbove)
			pdf.SetFont(fontFamily, "B", sectionSize)
			pdf.MultiCell(0, lineHeight, s.title, "", "C", false)
			pdf.Ln(1.5)
		}

		g.writeItems(pdf, s)
	}
}

// writeItems Выводит пункты раздела.
func (g *Generator) writeItems(pdf *fpdf.Fpdf, s section) {
	pdf.SetFont(fontFamily, "", bodySize)

	for _, it := range s.items {
		if it.text == "" {
			// Пустая строка в шаблоне — вертикальный отступ.
			pdf.Ln(lineHeight / 2)

			continue
		}

		text := it.text
		if it.number != "" {
			text = it.number + " " + text
		}

		pdf.MultiCell(0, lineHeight, text, "", "J", false)
		pdf.Ln(1.5)
	}
}
