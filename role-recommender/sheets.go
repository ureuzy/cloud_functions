package role_recommender

import (
	"context"
	"fmt"
	"google.golang.org/api/sheets/v4"
	"log"
)

type SheetsClient struct {
	*sheets.Service
}

type SpreadSheetsClient struct {
	*sheets.SpreadsheetsService
	sheetID   string
	sheetName string
}

func NewSheetsClient(ctx context.Context) (*SheetsClient, error) {
	srv, err := sheets.NewService(ctx)
	if err != nil {
		log.Printf("Unable to retrieve Sheets client: %v", err)
		return nil, err
	}

	return &SheetsClient{srv}, nil
}

func (s *SheetsClient) SetSheets(sheetsID, sheetsName string) *SpreadSheetsClient {
	return &SpreadSheetsClient{s.Spreadsheets, sheetsID, sheetsName}
}

func (s *SpreadSheetsClient) Append(valuerange *sheets.ValueRange) *sheets.SpreadsheetsValuesAppendCall {
	fmt.Println("write")
	return s.Values.Append(s.sheetID, s.sheetName, valuerange)
}

func (s *SpreadSheetsClient) Get() *sheets.SpreadsheetsValuesGetCall {
	return s.Values.Get(s.sheetID, s.sheetName)
}

func (s *SpreadSheetsClient) Update(valuerange *sheets.ValueRange) *sheets.SpreadsheetsValuesUpdateCall {
	fmt.Println("write")
	return s.Values.Update(s.sheetID, s.sheetName, valuerange)
}

func (s *SpreadSheetsClient) Clear(clearvaluerequest *sheets.ClearValuesRequest) *sheets.SpreadsheetsValuesClearCall {
	return s.Values.Clear(s.sheetID, s.sheetName, clearvaluerequest)
}

func (s *SpreadSheetsClient) Init() error {
	_, err := s.Clear(&sheets.ClearValuesRequest{}).Do()
	if err != nil {
		return err
	}
	_, err = s.Append(&sheets.ValueRange{Values: GetInitData()}).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return err
	}
	return nil
}

func GetInitData() [][]interface{} {
	header := make([]interface{}, 4)
	str := []string{"Resource Name", "Type", "Principal", "Role Recommend"}
	for i, s := range str {
		header[i] = s
	}
	value := make([][]interface{}, 1)
	value = append(value, header)
	return value
}
