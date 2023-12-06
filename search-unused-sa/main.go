package search_unused_sa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"log"
	"regexp"
	"time"

	"cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/samber/lo"
	"google.golang.org/api/iterator"
	"google.golang.org/api/policyanalyzer/v1"

	"github.com/ureuzy/cloud_functions/search-unused-sa/config"
)

type Activity = policyanalyzer.GoogleCloudPolicyanalyzerV1Activity
type ExtendPolicyanalyzerV1Activity struct {
	*Activity
	UnMarshaledActivity UnMarshaledActivity
}

func (s *ExtendPolicyanalyzerV1Activity) unUsedSA() bool {
	return s.UnMarshaledActivity.LastAuthenticatedTime == ""
}

func (s *ExtendPolicyanalyzerV1Activity) isUserCreatedSA(projectID string) bool {
	re := regexp.MustCompile(fmt.Sprintf("%s.iam.gserviceaccount.com$", projectID))
	return re.MatchString(s.FullResourceName)
}

func (s *ExtendPolicyanalyzerV1Activity) daysAfterCreation(days int) bool {
	t, err := time.Parse(time.RFC3339, s.Activity.ObservationPeriod.StartTime)
	if err != nil {
		log.Println(err)
	}
	return time.Now().After(t.AddDate(0, 0, days))
}

type ServiceAccountActivity struct {
	Activities []*ExtendPolicyanalyzerV1Activity
}

func (s *ServiceAccountActivity) debugPrint() {
	for _, activity := range s.Activities {
		fmt.Printf("----%s----\n", activity.Activity.FullResourceName)
		fmt.Printf("%s - %s\n", activity.Activity.ObservationPeriod.StartTime, activity.Activity.ObservationPeriod.EndTime)
		if activity.UnMarshaledActivity.LastAuthenticatedTime == "" {
			fmt.Println("Unused")
		} else {
			fmt.Printf("LastAuthenticatedTime: %s\n", activity.UnMarshaledActivity.LastAuthenticatedTime)
		}
	}
}

type UnMarshaledActivity struct {
	LastAuthenticatedTime string `json:"lastAuthenticatedTime"`
	ServiceAccount        `json:"serviceAccount"`
}

type ServiceAccount struct {
	ServiceAccountId string `json:"serviceAccountId"`
	FullResourceName string `json:"fullResourceName"`
	ProjectNumber    string `json:"projectNumber"`
}

type Option func(*ServiceAccountActivity) []*ExtendPolicyanalyzerV1Activity

func unUsedSA() Option {
	return func(activity *ServiceAccountActivity) []*ExtendPolicyanalyzerV1Activity {
		return lo.Filter(activity.Activities, func(item *ExtendPolicyanalyzerV1Activity, index int) bool {
			return item.unUsedSA()
		})
	}
}

func isUserCreatedSA(projectID string) Option {
	return func(activity *ServiceAccountActivity) []*ExtendPolicyanalyzerV1Activity {
		return lo.Filter(activity.Activities, func(item *ExtendPolicyanalyzerV1Activity, index int) bool {
			return item.isUserCreatedSA(projectID)
		})
	}
}

func daysAfterCreation(days int) Option {
	return func(activity *ServiceAccountActivity) []*ExtendPolicyanalyzerV1Activity {
		return lo.Filter(activity.Activities, func(item *ExtendPolicyanalyzerV1Activity, index int) bool {
			return item.daysAfterCreation(days)
		})
	}
}

func (s *ServiceAccountActivity) Filter(options []Option) *ServiceAccountActivity {
	for _, opt := range options {
		s.Activities = opt(s)
	}
	return s
}

func init() {
	functions.CloudEvent("Main", main)
}

func main(ctx context.Context, e event.Event) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}

	c, err := resourcemanager.NewProjectsClient(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	policyanalyzerService, err := policyanalyzer.NewService(ctx)
	if err != nil {
		return err
	}

	req := &resourcemanagerpb.SearchProjectsRequest{}
	it := c.SearchProjects(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}

		saActivities, err := saActivityAnalyze(policyanalyzerService, resp.ProjectId, "")
		if err != nil {
			log.Println(err)
			continue
		}
		filteredActivities := saActivities.Filter([]Option{
			unUsedSA(),
			isUserCreatedSA(resp.ProjectId),
			daysAfterCreation(conf.DaysAfterCreation),
		})
		filteredActivities.debugPrint()
	}
	return nil
}

func saActivityAnalyze(svc *policyanalyzer.Service, projectID string, nextToken string) (*ServiceAccountActivity, error) {
	parent := fmt.Sprintf("projects/%s/locations/asia-northeast1/activityTypes/serviceAccountLastAuthentication", projectID)
	queryCall := svc.Projects.Locations.ActivityTypes.Activities.Query(parent)
	res, err := queryCall.PageToken(nextToken).Do()
	if err != nil {
		return nil, err
	}

	result := unMarshalActivity(res.Activities)
	if res.NextPageToken == "" {
		return &ServiceAccountActivity{Activities: result}, nil
	}
	activities, err := saActivityAnalyze(svc, projectID, res.NextPageToken)
	if err != nil {
		return nil, err
	}
	return &ServiceAccountActivity{Activities: append(result, activities.Activities...)}, nil
}

func unMarshalActivity(activities []*policyanalyzer.GoogleCloudPolicyanalyzerV1Activity) []*ExtendPolicyanalyzerV1Activity {
	var result []*ExtendPolicyanalyzerV1Activity
	for i, activity := range activities {
		b, err := activity.Activity.MarshalJSON()
		if err != nil {
			log.Println(err)
			continue
		}
		var unMarshaledActivity UnMarshaledActivity
		err = json.Unmarshal(b, &unMarshaledActivity)
		if err != nil {
			log.Println(err)
			continue
		}
		result = append(result, &ExtendPolicyanalyzerV1Activity{
			Activity:            activities[i],
			UnMarshaledActivity: unMarshaledActivity,
		})
	}
	return result
}
