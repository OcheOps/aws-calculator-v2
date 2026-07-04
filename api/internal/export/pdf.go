package export

import (
	"io"
	"time"

	"github.com/OcheOps/aws-calculator/api/internal/estimates"
	"github.com/go-pdf/fpdf"
)

func WritePDF(w io.Writer, est *estimates.Estimate) error {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(est.Name, false)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 20)
	pdf.CellFormat(0, 12, est.Name, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, "Currency: "+est.Currency, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Generated: "+time.Now().UTC().Format(time.RFC3339), "", 1, "L", false, 0, "")

	pdf.Ln(4)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(90, 8, "Item", "1", 0, "L", false, 0, "")
	pdf.CellFormat(30, 8, "Region", "1", 0, "L", false, 0, "")
	pdf.CellFormat(20, 8, "Qty", "1", 0, "R", false, 0, "")
	pdf.CellFormat(30, 8, "Monthly", "1", 1, "R", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	for _, it := range est.Items {
		pdf.CellFormat(90, 7, LineSummary(it), "1", 0, "L", false, 0, "")
		pdf.CellFormat(30, 7, it.RegionCode, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 7, it.Quantity.String(), "1", 0, "R", false, 0, "")
		pdf.CellFormat(30, 7, it.MonthlyCostSnapshot.StringFixed(4), "1", 1, "R", false, 0, "")
	}
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(140, 8, "Total", "1", 0, "R", false, 0, "")
	pdf.CellFormat(30, 8, est.Total.StringFixed(4)+" "+est.Currency, "1", 1, "R", false, 0, "")

	return pdf.Output(w)
}
