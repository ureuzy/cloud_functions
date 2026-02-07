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
	TotalMonth float64
	DailyCosts []DailyCost
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

	err = sendToSlack(conf, costs)
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
	monthStartDate := now.Format("2006-01-01")

	queryStr := fmt.Sprintf(
		"SELECT CAST(SUM(cost + (SELECT SUM(IFNULL(c.amount, 0)) FROM UNNEST(credits) c)) AS STRING) as cost, " +
			"CAST(DATE(usage_start_time, 'Asia/Tokyo') AS STRING) as usage_date " +
			"FROM `%s` " +
			"WHERE DATE(usage_start_time, 'Asia/Tokyo') >= '%s' " +
			"GROUP BY usage_date",
		conf.BillingTable, monthStartDate,
	)

	useLegacySql := false
	res, err := svc.Jobs.Query(conf.ProjectID, &bigquery.QueryRequest{
		Query:        queryStr,
		UseLegacySql: &useLegacySql,
	}).Context(ctx).Do()

	if err != nil || (res != nil && len(res.Rows) == 0) {
		log.Printf("No real data available (err: %v).", err)
		return &CostData{
			TotalMonth: 0,
			DailyCosts: generateEmptyDailyCosts(now),
		},
	nil
	}

	var totalMonth float64
	dailyMap := make(map[string]float64)
	for _, row := range res.Rows {
		if len(row.F) < 2 {
			continue
		}
		costStr, _ := row.F[0].V.(string)
		usageDate, _ := row.F[1].V.(string)
		c, _ := strconv.ParseFloat(costStr, 64)
		totalMonth += c
		dailyMap[usageDate] = c
	}

	var dailyCosts []DailyCost
	for i := 1; i <= 7; i++ {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		cost := 0.0
		if val, ok := dailyMap[d]; ok {
			cost = val
		}
		dailyCosts = append(dailyCosts, DailyCost{Date: d, Cost: cost})
	}

	return &CostData{
		TotalMonth: totalMonth,
		DailyCosts: dailyCosts,
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

func sendToSlack(conf *config.Config, costs *CostData) error {
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

	return slack.PostWebhook(conf.SlackWebhookUrl, &slack.WebhookMessage{
		Username:    "Billing Monitor",
		IconEmoji:   ":moneybag:",
		Channel:     conf.Channel,
		Attachments: []slack.Attachment{{Color: "#34A853", Blocks: slack.Blocks{BlockSet: blocks}}},
	})
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
