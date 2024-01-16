package role_recommender

import (
	"context"
	"google.golang.org/api/sheets/v4"
	"log"
)

type SheetsClient struct {
	*sheets.Service
}

type SpreadSheetsClient struct {
	*sheets.SpreadsheetsService
	sheetID string
}

func NewSheetsClient(ctx context.Context) (*SheetsClient, error) {
	srv, err := sheets.NewService(ctx)
	if err != nil {
		log.Printf("Unable to retrieve Sheets client: %v", err)
		return nil, err
	}

	return &SheetsClient{srv}, nil
}

func (s *SheetsClient) SetSheetsID(sheetsID string) *SpreadSheetsClient {
	return &SpreadSheetsClient{s.Spreadsheets, sheetsID}
}

func (s *SpreadSheetsClient) Append(range_ string, valuerange *sheets.ValueRange) *sheets.SpreadsheetsValuesAppendCall {
	return s.Values.Append(s.sheetID, range_, valuerange)
}

func (s *SpreadSheetsClient) Get(range_ string) *sheets.SpreadsheetsValuesGetCall {
	return s.Values.Get(s.sheetID, range_)
}

func (s *SpreadSheetsClient) Update(range_ string, valuerange *sheets.ValueRange) *sheets.SpreadsheetsValuesUpdateCall {
	return s.Values.Update(s.sheetID, range_, valuerange)
}

func (s *SpreadSheetsClient) Clear(range_ string, clearvaluerequest *sheets.ClearValuesRequest) *sheets.SpreadsheetsValuesClearCall {
	return s.Values.Clear(s.sheetID, range_, clearvaluerequest)
}

func (s *SpreadSheetsClient) Init(range_ string) error {
	_, err := s.Clear(range_, &sheets.ClearValuesRequest{}).Do()
	if err != nil {
		return err
	}
	_, err = s.Append(range_, &sheets.ValueRange{Values: GetInitData()}).
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
