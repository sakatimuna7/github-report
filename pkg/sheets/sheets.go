package sheets

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/sheets/v4"
)

var indonesianMonths = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
var indonesianDays = []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}

var indonesianMonthsCaps = []string{"JANUARI", "FEBRUARI", "MARET", "APRIL", "MEI", "JUNI", "JULI", "AGUSTUS", "SEPTEMBER", "OKTOBER", "NOVEMBER", "DESEMBER"}

func getFormattedDate(t time.Time) string {
	dayName := indonesianDays[t.Weekday()]
	monthName := indonesianMonths[t.Month()-1]
	return fmt.Sprintf("%s, %d %s %d", dayName, t.Day(), monthName, t.Year())
}

func getSheetName(divisi string, t time.Time) string {
	if divisi == "" {
		divisi = "FRONTEND" // default
	}
	return fmt.Sprintf("%s-%s", strings.ToUpper(divisi), indonesianMonthsCaps[t.Month()-1])
}

func WriteReportToSheet(srv *sheets.Service, spreadsheetId string, divisi string, developerName string, reportDate time.Time, reportContent string) error {
	sheetName := getSheetName(divisi, reportDate)
	targetDateStr := getFormattedDate(reportDate)
	developerName = strings.ToUpper(strings.TrimSpace(developerName))

	// Get the values from columns B and C (TANGGAL and NAMA)
	readRange := fmt.Sprintf("'%s'!B:C", sheetName)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
	if err != nil {
		return fmt.Errorf("unable to retrieve data from sheet %s: %v", sheetName, err)
	}

	if len(resp.Values) == 0 {
		return fmt.Errorf("no data found in sheet %s", sheetName)
	}

	var targetRow int = -1
	var lastSeenDate string

	for i, row := range resp.Values {
		// Row is 0-indexed in array, but Sheets uses 1-indexed. So row number is i+1.
		
		var dateVal string
		if len(row) > 0 {
			dateVal = fmt.Sprintf("%v", row[0])
		}
		var nameVal string
		if len(row) > 1 {
			nameVal = fmt.Sprintf("%v", row[1])
		}

		if dateVal != "" {
			lastSeenDate = dateVal
		}

		if lastSeenDate == targetDateStr && strings.ToUpper(strings.TrimSpace(nameVal)) == developerName {
			targetRow = i + 1
			break
		}
	}

	if targetRow == -1 {
		return fmt.Errorf("could not find row for date '%s' and developer '%s'", targetDateStr, developerName)
	}

	// Column D is PROGRESS TERKINI
	writeRange := fmt.Sprintf("'%s'!D%d", sheetName, targetRow)
	var vr sheets.ValueRange
	vr.Values = append(vr.Values, []interface{}{reportContent})

	_, err = srv.Spreadsheets.Values.Update(spreadsheetId, writeRange, &vr).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return fmt.Errorf("unable to write to sheet: %v", err)
	}

	return nil
}
