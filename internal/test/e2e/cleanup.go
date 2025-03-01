package e2e

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/eks-anywhere/internal/pkg/ec2"
	"github.com/aws/eks-anywhere/internal/pkg/s3"
	"github.com/aws/eks-anywhere/pkg/logger"
	"github.com/aws/eks-anywhere/pkg/validations"
)

func CleanUpAwsTestResources(storageBucket string, maxAge string, tag string) error {
	session, err := session.NewSession()
	if err != nil {
		return fmt.Errorf("error creating session: %v", err)
	}
	logger.V(1).Info("Fetching list of EC2 instances")
	key := "Integration-Test"
	value := tag
	maxAgeFloat, err := strconv.ParseFloat(maxAge, 64)
	if err != nil {
		return fmt.Errorf("error parsing max age: %v", err)
	}
	results, err := ec2.ListInstances(session, key, value, maxAgeFloat)
	if err != nil {
		return fmt.Errorf("error listing EC2 instances: %v", err)
	}
	logger.V(1).Info("Successfully listed EC2 instances for termination")
	if len(results) != 0 {
		logger.V(1).Info("Terminating EC2 instances")
		err = ec2.TerminateEc2Instances(session, results)
		if err != nil {
			return fmt.Errorf("error terminating EC2 instacnes: %v", err)
		}
		logger.V(1).Info("Successfully terminated EC2 instances")
	} else {
		logger.V(1).Info("No EC2 instances available for termination")
	}
	logger.V(1).Info("Clean up s3 bucket objects")
	err = s3.CleanUpS3Bucket(session, storageBucket, maxAgeFloat)
	if err != nil {
		return fmt.Errorf("error clean up s3 bucket objects: %v", err)
	}
	logger.V(1).Info("Successfully cleaned up s3 bucket")

	return nil
}

func CleanUpVsphereTestResources(ctx context.Context, clusterName string) error {
	clusterName, err := validations.ValidateClusterNameArg([]string{clusterName})
	if err != nil {
		return fmt.Errorf("error validating cluster name: %v", err)
	}
	err = vsphereRmVms(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("error removing vcenter vms: %v", err)
	}
	logger.V(1).Info("Vsphere vcenter vms cleanup complete")
	return nil
}
