package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws/request"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/pkg/errors"
)

// From https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/EventTypes.html#SSM-Parameter-Store-event-types
type EventDetail struct {
	Operation   string `json:"operation"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type TagLister interface {
	ListTagsForResourceWithContext(aws.Context, *ssm.ListTagsForResourceInput, ...request.Option) (*ssm.ListTagsForResourceOutput, error)
}

type ServiceDeployer interface {
	UpdateServiceWithContext(aws.Context, *ecs.UpdateServiceInput, ...request.Option) (*ecs.UpdateServiceOutput, error)
}

// Looks up `restarts` tag of changed parameter in parameter store.
// The `restarts` tag may include pairs of cluster+service strings to notify this lambda function of services in clusters
// to be restarted on parameter change.
// A valid `restarts` tag value may look like: `cluster_a:service_a,cluster_b:service_b,cluster_a:service_c
func RequestHandler(lister TagLister, deployer ServiceDeployer) func(aws.Context, events.CloudWatchEvent) error {
	return func(ctx aws.Context, event events.CloudWatchEvent) error {

		var detail EventDetail
		err := json.Unmarshal(event.Detail, &detail)
		if err != nil {
			return errors.Wrap(err, "error decoding event detail")
		}

		// if parameter was updated and part of restart parameter list
		if detail.Operation == "Update" {
			var toRestart []RestartableClusterService
			tagResp, err := lister.ListTagsForResourceWithContext(ctx, &ssm.ListTagsForResourceInput{
				ResourceId:   &detail.Name,
				ResourceType: aws.String("Parameter"),
			})
			if err != nil {
				return errors.Wrapf(err, "error reading tags for parameter %q", detail.Name)
			}

			for _, tag := range tagResp.TagList {
				if *tag.Key == "restarts" {
					services := strings.Split(*tag.Value, " ")
					if len(services) == 0 {
						return errors.New(`no services configured in tag "restarts"`)
					}
					for _, service := range services {
						serviceClusterSplit := strings.Split(service, ":")
						if len(serviceClusterSplit) != 2 {
							return fmt.Errorf("didnt find cluster and service split by ':' in %q", service)
						}
						toRestart = append(toRestart, RestartableClusterService{Cluster: serviceClusterSplit[0], Service: serviceClusterSplit[1]})
					}

				}
			}

			for _, service := range toRestart {
				log.Printf("Redeploying service %q in cluster %q after update of parameter %q", service.Service, service.Cluster, detail.Name)
				_, err := deployer.UpdateServiceWithContext(ctx, &ecs.UpdateServiceInput{
					Cluster:            &service.Cluster,
					Service:            &service.Service,
					ForceNewDeployment: aws.Bool(true),
				})
				if err != nil {
					return errors.Wrap(err, "service updated unsuccessful")
				}
			}
		}

		return nil
	}
}

type RestartableClusterService struct {
	Cluster string
	Service string
}

func main() {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(endpoints.UsEast1RegionID),
	}))
	ssmClient := ssm.New(sess)
	ecsClient := ecs.New(sess)
	lambda.Start(RequestHandler(ssmClient, ecsClient))
}
