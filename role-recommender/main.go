package role_recommender

import (
	"context"
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"log"
	"strings"

	recommender "cloud.google.com/go/recommender/apiv1"
	"cloud.google.com/go/recommender/apiv1/recommenderpb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
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
	sheetSrv := sheetsClient.SetSheets(conf.SheetID, "シート1")
	if err = sheetSrv.Init(); err != nil {
		return err
	}

	organization := organizationsClient.SearchOrganizations(ctx, organizationsReq)
	data := SheetData{}
	for {
		resource, err := organization.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			log.Println(err)
			break
		}
		r := recommend(ctx, recommenderClient, resource)
		if len(r.RecommendsMap) == 0 {
			continue
		}
		data.merge(r.toSheetData())
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
		r := recommend(ctx, recommenderClient, resource)
		if len(r.RecommendsMap) == 0 {
			continue
		}
		data.merge(r.toSheetData())
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
		r := recommend(ctx, recommenderClient, resource)
		if len(r.RecommendsMap) == 0 {
			continue
		}
		data.merge(r.toSheetData())

	}
	run(sheetSrv, data)

	return nil
}

func run(sheetSrv *SpreadSheetsClient, data [][]interface{}) {
	_, err := sheetSrv.Append(&sheets.ValueRange{Values: data}).
		ValueInputOption("RAW").
		Do()
	if err != nil {
		log.Println(err)
		return
	}
}

func recommend(ctx context.Context, client *recommender.Client, resource Resource) Hoge {
	var result Recommends

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

	return Hoge{
		Resource:      resource,
		RecommendsMap: result.groupByMember(),
	}
}
