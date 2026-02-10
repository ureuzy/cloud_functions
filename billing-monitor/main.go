package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"google.golang.org/api/bigquery/v2"
	"google.golang.org/api/option"

	"github.com/ureuzy/cloud_functions/billing-monitor/config"
)

type DailyCost struct {
	Date string
	Cost float64
}

type CostData struct {
	TotalMonth   float64
	DailyCosts   []DailyCost
	ServiceCosts []ServiceCost
}

type ServiceCost struct {
	Service string
	Cost    float64
}

func main() {
	ctx := context.Background()
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting Billing Monitor (Project: %s)", conf.ProjectID)

	costs, err := fetchCosts(ctx, conf)
	if err != nil {
		log.Fatalf("Failed to fetch costs: %v", err)
	}

	slackClient := slack.New(conf.SlackBotToken)
	err = sendToSlack(slackClient, conf, costs)
	if err != nil {
		log.Fatalf("Failed to send to Slack: %v", err)
	}

	log.Println("Job completed successfully!")
}

func fetchCosts(ctx context.Context, conf *config.Config) (*CostData, error) {
	svc, err := bigquery.NewService(ctx, option.WithQuotaProject(conf.ProjectID))
	if err != nil {
		return nil, fmt.Errorf("bigquery.NewService: %v", err)
	}

	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	now := time.Now().In(jst)
	// 今月の1日を計算
	monthStartDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, jst).Format("2006-01-02")

	queryStr := fmt.Sprintf(
		"SELECT CAST(SUM(cost) AS STRING) as cost, " +
			"CAST(DATE(usage_start_time, 'Asia/Tokyo') AS STRING) as usage_date " +
			"FROM `%s` " +
			"WHERE DATE(usage_start_time, 'Asia/Tokyo') >= '%s' " +
			"GROUP BY usage_date " +
			"ORDER BY usage_date",
		conf.BillingTable, monthStartDate,
	)

	log.Printf("Executing query: %s", queryStr)
	log.Printf("Month start date: %s", monthStartDate)

	useLegacySql := false
	res, err := svc.Jobs.Query(conf.ProjectID, &bigquery.QueryRequest{
		Query:        queryStr,
		UseLegacySql: &useLegacySql,
	}).Context(ctx).Do()

	if err != nil {
		log.Printf("Query error: %v", err)
		return &CostData{
			TotalMonth: 0,
			DailyCosts: generateEmptyDailyCosts(now),
		}, nil
	}

	if res != nil && len(res.Rows) == 0 {
		log.Printf("No data returned from BigQuery (empty result set)")
		return &CostData{
			TotalMonth: 0,
			DailyCosts: generateEmptyDailyCosts(now),
		},
	nil
	}

	log.Printf("Query returned %d rows", len(res.Rows))

	var totalMonth float64
	dailyMap := make(map[string]float64)
	for _, row := range res.Rows {
		if len(row.F) < 2 {
			continue
		}
		costStr, _ := row.F[0].V.(string)
		usageDate, _ := row.F[1].V.(string)
		c, _ := strconv.ParseFloat(costStr, 64)
		log.Printf("Data: date=%s, cost=%f", usageDate, c)
		totalMonth += c
		dailyMap[usageDate] = c
	}

	log.Printf("Total month cost: %f", totalMonth)

	var dailyCosts []DailyCost
	for i := 1; i <= 7; i++ {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		cost := 0.0
		if val, ok := dailyMap[d]; ok {
			cost = val
		}
		dailyCosts = append(dailyCosts, DailyCost{Date: d, Cost: cost})
	}

	// サービス別コストを取得
	serviceCosts, err := fetchServiceCosts(ctx, svc, conf, monthStartDate)
	if err != nil {
		log.Printf("Failed to fetch service costs: %v", err)
		// サービス別取得に失敗してもメイン処理は続行
		serviceCosts = []ServiceCost{}
	}

	return &CostData{
		TotalMonth:   totalMonth,
		DailyCosts:   dailyCosts,
		ServiceCosts: serviceCosts,
	},
	nil
}

func generateEmptyDailyCosts(now time.Time) []DailyCost {
	var dailyCosts []DailyCost
	for i := 1; i <= 7; i++ {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		dailyCosts = append(dailyCosts, DailyCost{Date: d, Cost: 0})
	}
	return dailyCosts
}

func fetchServiceCosts(ctx context.Context, svc *bigquery.Service, conf *config.Config, monthStartDate string) ([]ServiceCost, error) {
	queryStr := fmt.Sprintf(
		"SELECT service.description as service, SUM(cost) as total_cost " +
			"FROM `%s` " +
			"WHERE DATE(usage_start_time, 'Asia/Tokyo') >= '%s' " +
			"GROUP BY service " +
			"ORDER BY total_cost DESC",
		conf.BillingTable, monthStartDate,
	)

	log.Printf("Executing service cost query: %s", queryStr)

	useLegacySql := false
	res, err := svc.Jobs.Query(conf.ProjectID, &bigquery.QueryRequest{
		Query:        queryStr,
		UseLegacySql: &useLegacySql,
	}).Context(ctx).Do()

	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}

	if res == nil || len(res.Rows) == 0 {
		log.Printf("No service cost data returned")
		return []ServiceCost{}, nil
	}

	log.Printf("Service cost query returned %d rows", len(res.Rows))

	var serviceCosts []ServiceCost
	for _, row := range res.Rows {
		if len(row.F) < 2 {
			continue
		}
		serviceName, _ := row.F[0].V.(string)

		// BigQueryの数値型はinterface{}で返ってくるので、型アサーションが必要
		var cost float64
		switch v := row.F[1].V.(type) {
		case float64:
			cost = v
		case string:
			cost, _ = strconv.ParseFloat(v, 64)
		}

		log.Printf("Service: %s, cost: %f", serviceName, cost)
		serviceCosts = append(serviceCosts, ServiceCost{Service: serviceName, Cost: cost})
	}

	return serviceCosts, nil
}

func sendToSlack(slackClient *slack.Client, conf *config.Config, costs *CostData) error {
	var maxCost float64
	for _, dc := range costs.DailyCosts {
		if dc.Cost > maxCost {
			maxCost = dc.Cost
		}
	}
	if maxCost == 0 {
		maxCost = 1
	}

	barWidth := 15
	var lines []string
	for _, dc := range costs.DailyCosts {
		label := dc.Date[5:] // MM-DD
		bar := createProgressBar(dc.Cost, maxCost, barWidth)
		lines = append(lines, fmt.Sprintf("`%s` `%s` ¥%6.0f", label, bar, dc.Cost))
	}

	content := fmt.Sprintf(
		"• *当月累計*: ¥%.2f\n\n*直近7日間の推移*\n%s",
		costs.TotalMonth,
		strings.Join(lines, "\n"),
	)

	blocks := []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", ":money_with_wings: *Google Cloud Cost Report*", false, false), nil, nil),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", content, false, false), nil, nil),
	}

	// メインメッセージを投稿
	_, threadTs, err := slackClient.PostMessage(conf.Channel,
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionUsername("Billing Monitor"),
		slack.MsgOptionIconEmoji(":moneybag:"),
	)
	if err != nil {
		return fmt.Errorf("failed to post main message: %v", err)
	}

	log.Printf("Posted main message with thread_ts: %s", threadTs)

	// サービス別コストをスレッドに投稿
	if len(costs.ServiceCosts) > 0 {
		var serviceLines []string
		for i, sc := range costs.ServiceCosts {
			if i >= 10 {
				// 上位10件のみ表示
				break
			}
			serviceLines = append(serviceLines, fmt.Sprintf("%d. *%s*: ¥%.2f", i+1, sc.Service, sc.Cost))
		}

		serviceContent := fmt.Sprintf("*サービス別コスト（当月累計・上位10件）*\n\n%s", strings.Join(serviceLines, "\n"))

		_, _, err = slackClient.PostMessage(conf.Channel,
			slack.MsgOptionText(serviceContent, false),
			slack.MsgOptionTS(threadTs),
		)
		if err != nil {
			log.Printf("Failed to post service breakdown: %v", err)
			// スレッド投稿失敗してもメイン処理は成功とする
		} else {
			log.Printf("Posted service breakdown to thread")
		}
	}

	return nil
}

func createProgressBar(value, max float64, width int) string {
	fraction := value / max
	filled := int(fraction * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	res := ""
	for i := 0; i < filled; i++ {
		res += "█"
	}
	for i := filled; i < width; i++ {
		res += "░"
	}
	return res
}
