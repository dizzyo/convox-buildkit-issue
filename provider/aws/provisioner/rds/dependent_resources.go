package rds

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
)

func (p *Provisioner) GetVPCCIDR(vpcID string) (string, error) {
	result, err := p.ec2Client.DescribeVpcs(context.Background(), &ec2.DescribeVpcsInput{
		VpcIds: []string{vpcID},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe VPC: %s", err)
	}

	if len(result.Vpcs) == 0 {
		return "", fmt.Errorf("no VPC found with ID %s", vpcID)
	}

	return *result.Vpcs[0].CidrBlock, nil
}

func (p *Provisioner) GetSecurityGroupByTags(tags map[string]string) ([]ec2types.SecurityGroup, error) {
	var filters []ec2types.Filter
	for key, value := range tags {
		filters = append(filters, ec2types.Filter{
			Name:   aws.String("tag:" + key),
			Values: []string{value},
		})
	}
	input := &ec2.DescribeSecurityGroupsInput{
		Filters: filters,
	}

	result, err := p.ec2Client.DescribeSecurityGroups(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe security groups: %s", err)
	}

	return result.SecurityGroups, nil
}

func (p *Provisioner) GetDBSubnetGroup(subnetGroupName string) (*rdstypes.DBSubnetGroup, error) {
	input := &rds.DescribeDBSubnetGroupsInput{
		DBSubnetGroupName: &subnetGroupName,
	}

	result, err := p.rdsClient.DescribeDBSubnetGroups(context.Background(), input)
	if err != nil {
		if err, ok := err.(awserr.Error); ok && err.Code() == "DBSubnetGroupNotFoundFault" {
			return nil, fmt.Errorf("db subnet group not found")
		}
		return nil, fmt.Errorf("failed to describe db subnet groups: %s", err)
	}

	if len(result.DBSubnetGroups) == 0 {
		return nil, fmt.Errorf("db subnet group not found")
	}

	return &result.DBSubnetGroups[0], nil
}

func (p *Provisioner) CreateSecurityGroup(groupName, vpcID, provisionID string) (string, error) {
	input := &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(groupName),
		Description: aws.String("db security group"),
		VpcId:       aws.String(vpcID),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeSecurityGroup,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String(ProvisionerName),
						Value: aws.String(provisionID),
					},
				},
			},
		},
	}

	result, err := p.ec2Client.CreateSecurityGroup(context.Background(), input)
	if err != nil {
		return "", fmt.Errorf("failed to create security group: %s", err)
	}

	securityGroupID := *result.GroupId

	return securityGroupID, nil
}

func (p *Provisioner) AddIngressRule(securityGroupID string, port int32, vpcCidr string) error {
	input := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(securityGroupID),
		IpPermissions: []ec2types.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(port),
				ToPort:     aws.Int32(port),
				IpRanges: []ec2types.IpRange{
					{
						CidrIp: aws.String(vpcCidr),
					},
				},
			},
		},
	}

	_, err := p.ec2Client.AuthorizeSecurityGroupIngress(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to add ingress rule: %s", err)
	}

	return nil
}

func (p *Provisioner) CreateDBSubnetGroup(groupName string, subnetIDs []string, provisionID string) (string, error) {
	input := &rds.CreateDBSubnetGroupInput{
		DBSubnetGroupName:        aws.String(groupName),
		DBSubnetGroupDescription: aws.String("db subnet group"),
		SubnetIds:                subnetIDs,
		Tags: []rdstypes.Tag{
			{
				Key:   aws.String(ProvisionerName),
				Value: aws.String(provisionID),
			},
		},
	}

	result, err := p.rdsClient.CreateDBSubnetGroup(context.Background(), input)
	if err != nil {
		return "", fmt.Errorf("failed to create DB subnet group: %w", err)
	}

	return *result.DBSubnetGroup.DBSubnetGroupName, nil
}

func (p *Provisioner) DeleteSecurityGroup(groupId string) error {
	_, err := p.ec2Client.DeleteSecurityGroup(context.Background(), &ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(groupId),
	})
	return err
}

func (p *Provisioner) DeleteDBSubnetGroup(subnetGroupName string) error {
	_, err := p.rdsClient.DeleteDBSubnetGroup(context.Background(), &rds.DeleteDBSubnetGroupInput{
		DBSubnetGroupName: aws.String(subnetGroupName),
	})
	return err
}