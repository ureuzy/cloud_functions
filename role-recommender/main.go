package role_recommender

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	recommender "cloud.google.com/go/recommender/apiv1"
	"cloud.google.com/go/recommender/apiv1/recommenderpb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/samber/lo"
	"github.com/ureuzy/cloud_functions/role-recommender/config"
	"google.golang.org/api/iterator"
	"google.golang.org/api/sheets/v4"
)

func init() {
	functions.CloudEvent("Main", main)
}

func main(ctx context.Context, e event.Event) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}

	recommenderClient, err := recommender.NewClient(ctx)
	if err != nil {
		return err
	}

	organizationsClient, err := resourcemanager.NewOrganizationsClient(ctx)
	if err != nil {
		return err
	}
	organizationsReq := &resourcemanagerpb.SearchOrganizationsRequest{}

	foldersClient, err := resourcemanager.NewFoldersClient(ctx)
	if err != nil {
		return err
	}
	foldersReq := &resourcemanagerpb.SearchFoldersRequest{}

	projectsClient, err := resourcemanager.NewProjectsClient(ctx)
	if err != nil {
		return err
	}
	projectsReq := &resourcemanagerpb.SearchProjectsRequest{}

	defer func() {
		recommenderClient.Close()
		organizationsClient.Close()
		foldersClient.Close()
		projectsClient.Close()
	}()

	sheetsClient, err := NewSheetsClient(ctx)
	if err != nil {
		return err
	}
	sheetSrv := sheetsClient.SetSheetsID(conf.SheetID)
	if err := sheetSrv.Init("シート1"); err != nil {
		return err
	}

	organization := organizationsClient.SearchOrganizations(ctx, organizationsReq)
	for {
		resource, err := organization.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			log.Println(err)
			break
		}
		run(sheetSrv, recommend(ctx, recommenderClient, resource), resource)
	}

	folder := foldersClient.SearchFolders(ctx, foldersReq)
	for {
		resource, err := folder.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			log.Println(err)
			break
		}
		run(sheetSrv, recommend(ctx, recommenderClient, resource), resource)
	}

	project := projectsClient.SearchProjects(ctx, projectsReq)
	for {
		resource, err := project.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			log.Println(err)
			break
		}
		if strings.HasPrefix(resource.ProjectId, "sys-") {
			continue
		}
		run(sheetSrv, recommend(ctx, recommenderClient, resource), resource)
	}
	return nil
}

type Recommend struct {
	Role   string
	Member string
}

type RecommendMap map[string][]Recommend

func (r *RecommendMap) toSheetData(resourceName string, ID string) [][]interface{} {
	var sheet [][]interface{}
	for k, v := range *r {
		col := make([]interface{}, 4)
		var roles string
		for _, s := range v {
			roles += fmt.Sprintf("%s\n", s.Role)
		}
		for i, s := range []string{resourceName, strings.Split(ID, "/")[0], k, roles} {
			col[i] = s
		}
		sheet = append(sheet, col)
	}
	return sheet
}

type Resource interface {
	GetName() string
	GetDisplayName() string
}

func run(sheetSrv *SpreadSheetsClient, data RecommendMap, resource Resource) {
	d := data.toSheetData(resource.GetDisplayName(), resource.GetName())
	if len(data) == 0 {
		return
	}
	_, err := sheetSrv.Append("シート1", &sheets.ValueRange{Values: d}).
		ValueInputOption("RAW").
		Do()
	if err != nil {
		log.Println(err)
		return
	}
}

func recommend(ctx context.Context, client *recommender.Client, resource Resource) RecommendMap {
	var result []Recommend

	req := &recommenderpb.ListRecommendationsRequest{
		Parent: fmt.Sprintf("%s/locations/global/recommenders/google.iam.policy.Recommender", resource.GetName()),
	}
	it := client.ListRecommendations(ctx, req)

	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			log.Println(err)
			break
		}
		for _, groups := range resp.Content.OperationGroups {
			for _, operation := range groups.Operations {
				if operation.Action == "add" {
					result = append(result, Recommend{
						Role:   fmt.Sprintf("+ %s", operation.PathFilters["/iamPolicy/bindings/*/role"].GetStringValue()),
						Member: operation.GetValue().GetStringValue(),
					})
				}
				if operation.Action == "remove" {
					result = append(result, Recommend{
						Role:   fmt.Sprintf("-  %s", operation.PathFilters["/iamPolicy/bindings/*/role"].GetStringValue()),
						Member: operation.PathFilters["/iamPolicy/bindings/*/members/*"].GetStringValue(),
					})
				}
			}
		}
	}
	return lo.GroupBy(result, func(item Recommend) string {
		return item.Member
	})
}
