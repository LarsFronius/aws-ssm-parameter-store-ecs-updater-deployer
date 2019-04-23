package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockLister struct {
	called int
	out    ssm.ListTagsForResourceOutput
}

func (m *MockLister) ListTagsForResourceWithContext(aws.Context, *ssm.ListTagsForResourceInput, ...request.Option) (*ssm.ListTagsForResourceOutput, error) {
	m.called++
	return &m.out, nil
}

type MockDeployer struct {
	called int
}

func (d *MockDeployer) UpdateServiceWithContext(aws.Context, *ecs.UpdateServiceInput, ...request.Option) (*ecs.UpdateServiceOutput, error) {
	d.called++
	return &ecs.UpdateServiceOutput{}, nil
}

func TestRequestHandler(t *testing.T) {
	testCases := []struct {
		name                 string
		event                events.CloudWatchEvent
		expectRetErr         bool
		expectDeployerCalled int
		expectListerCalled   int
		mockListerOutput     ssm.ListTagsForResourceOutput
		expectLogOutContains string
	}{
		{
			name:         "invalid event",
			event:        events.CloudWatchEvent{},
			expectRetErr: true,
		},
		{
			name:                 "with updated parameter not tagged with restarts",
			event:                events.CloudWatchEvent{Detail: json.RawMessage([]byte(`{"operation":"Update","name":"foo"}`))},
			expectRetErr:         false,
			expectDeployerCalled: 0,
			expectListerCalled:   1,
		},
		{
			name:                 "with updated parameter tagged with restarts",
			event:                events.CloudWatchEvent{Detail: json.RawMessage([]byte(`{"operation":"Update","name":"foo"}`))},
			expectRetErr:         false,
			mockListerOutput:     ssm.ListTagsForResourceOutput{TagList: []*ssm.Tag{{Key: aws.String("restarts"), Value: aws.String("bar:foo")}}},
			expectDeployerCalled: 1,
			expectListerCalled:   1,
			expectLogOutContains: `Redeploying service "foo" in cluster "bar" after update of parameter "foo"`,
		},
		{
			name:                 "with updated parameter tagged with multiple restarts",
			event:                events.CloudWatchEvent{Detail: json.RawMessage([]byte(`{"operation":"Update","name":"foo"}`))},
			expectRetErr:         false,
			mockListerOutput:     ssm.ListTagsForResourceOutput{TagList: []*ssm.Tag{{Key: aws.String("restarts"), Value: aws.String("bar:foo bar_2:foo_2")}}},
			expectDeployerCalled: 2,
			expectListerCalled:   1,
			expectLogOutContains: `Redeploying service "foo" in cluster "bar" after update of parameter "foo"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logBuffer := bytes.Buffer{}
			log.SetOutput(&logBuffer)
			lister := &MockLister{out: tc.mockListerOutput}
			deployer := &MockDeployer{}
			requestHandler := RequestHandler(lister, deployer)

			err := requestHandler(context.Background(), tc.event)
			if tc.expectRetErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expectListerCalled, lister.called)
			assert.Equal(t, tc.expectDeployerCalled, deployer.called)
			assert.Contains(t, logBuffer.String(), tc.expectLogOutContains)
		})
	}
}
